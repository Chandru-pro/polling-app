package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"pollingapp/config"
	"pollingapp/models"
	"pollingapp/ws"
)

type PollHandler struct {
	DB    *mongo.Database
	Redis *redis.Client
	Hub   *ws.Hub
	Cfg   *config.Config
}

func countsKey(pollID string) string  { return "poll:" + pollID + ":counts" }
func votersKey(pollID string) string  { return "poll:" + pollID + ":voters" }

type createPollRequest struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

// CreatePoll validates everything server-side: a real question, at least two
// non-blank, de-duplicated options, and a sane upper bound so nobody hands
// us a 10,000-option poll.
func (h *PollHandler) CreatePoll(c *gin.Context) {
	var req createPollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" || len(req.Question) > 300 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "question must be 1-300 characters"})
		return
	}

	seen := map[string]bool{}
	cleanOptions := []string{}
	for _, o := range req.Options {
		o = strings.TrimSpace(o)
		if o == "" || len(o) > 120 {
			continue
		}
		key := strings.ToLower(o)
		if seen[key] {
			continue
		}
		seen[key] = true
		cleanOptions = append(cleanOptions, o)
	}
	if len(cleanOptions) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll needs at least 2 distinct, non-empty options"})
		return
	}
	if len(cleanOptions) > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll supports at most 10 options"})
		return
	}

	userID := c.GetString("userId")

	options := make([]models.Option, len(cleanOptions))
	for i, text := range cleanOptions {
		options[i] = models.Option{Index: i, Text: text}
	}

	poll := models.Poll{
		ID:        uuid.NewString(),
		Question:  req.Question,
		Options:   options,
		CreatorID: userID,
		IsClosed:  false,
		CreatedAt: time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := h.DB.Collection("polls").InsertOne(ctx, poll); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create poll"})
		return
	}

	// Seed the Redis counters up front so reads never race against the first vote.
	pipe := h.Redis.Pipeline()
	for i := range options {
		pipe.HSet(ctx, countsKey(poll.ID), strconv.Itoa(i), 0)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not initialize live counters"})
		return
	}

	c.JSON(http.StatusCreated, poll)
}

func (h *PollHandler) GetPoll(c *gin.Context) {
	pollID := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var poll models.Poll
	err := h.DB.Collection("polls").FindOne(ctx, bson.M{"_id": pollID}).Decode(&poll)
	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch poll"})
		return
	}
	c.JSON(http.StatusOK, poll)
}

// ListMyPolls lets a logged-in creator see the polls they own.
func (h *PollHandler) ListMyPolls(c *gin.Context) {
	userID := c.GetString("userId")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cur, err := h.DB.Collection("polls").Find(ctx, bson.M{"creatorId": userID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list polls"})
		return
	}
	defer cur.Close(ctx)

	polls := []models.Poll{}
	if err := cur.All(ctx, &polls); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read polls"})
		return
	}
	c.JSON(http.StatusOK, polls)
}

// getResultsFromRedis reads the live counts. If Redis has nothing (cold
// start / eviction), it rebuilds from the durable Mongo vote log and warms
// the cache back up — Redis stays the fast path, Mongo is the safety net.
func (h *PollHandler) getResults(ctx context.Context, poll models.Poll) (models.PollResults, error) {
	vals, err := h.Redis.HGetAll(ctx, countsKey(poll.ID)).Result()
	counts := make([]int, len(poll.Options))

	if err == nil && len(vals) > 0 {
		for k, v := range vals {
			idx, convErr := strconv.Atoi(k)
			if convErr != nil || idx < 0 || idx >= len(counts) {
				continue
			}
			n, _ := strconv.Atoi(v)
			counts[idx] = n
		}
	} else {
		// Cold cache: rebuild from Mongo and reseed Redis.
		cur, aggErr := h.DB.Collection("votes").Aggregate(ctx, []bson.M{
			{"$match": bson.M{"pollId": poll.ID}},
			{"$group": bson.M{"_id": "$optionIdx", "count": bson.M{"$sum": 1}}},
		})
		if aggErr == nil {
			defer cur.Close(ctx)
			pipe := h.Redis.Pipeline()
			for cur.Next(ctx) {
				var row struct {
					ID    int `bson:"_id"`
					Count int `bson:"count"`
				}
				if decErr := cur.Decode(&row); decErr == nil && row.ID >= 0 && row.ID < len(counts) {
					counts[row.ID] = row.Count
					pipe.HSet(ctx, countsKey(poll.ID), strconv.Itoa(row.ID), row.Count)
				}
			}
			_, _ = pipe.Exec(ctx)
		}
	}

	total := 0
	for _, n := range counts {
		total += n
	}
	return models.PollResults{PollID: poll.ID, Counts: counts, Total: total}, nil
}

func (h *PollHandler) GetResults(c *gin.Context) {
	pollID := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var poll models.Poll
	if err := h.DB.Collection("polls").FindOne(ctx, bson.M{"_id": pollID}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}

	results, err := h.getResults(ctx, poll)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not compute results"})
		return
	}
	c.JSON(http.StatusOK, results)
}

type voteRequest struct {
	OptionIdx int    `json:"optionIdx"`
	VoterID   string `json:"voterId"`
}

// Vote is the hot path. Never trust the client: the poll must exist, the
// option index must be in range, the poll must be open, and the voter must
// not have voted on this poll before. Redis's SADD does the dedup check and
// the write atomically (first writer wins the race); Mongo's unique index
// on (pollId, voterId) is the second line of defense if two requests somehow
// land on different Redis nodes in a cluster deployment.
func (h *PollHandler) Vote(c *gin.Context) {
	pollID := c.Param("id")
	var req voteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	req.VoterID = strings.TrimSpace(req.VoterID)
	if req.VoterID == "" || len(req.VoterID) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid voterId"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var poll models.Poll
	if err := h.DB.Collection("polls").FindOne(ctx, bson.M{"_id": pollID}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}
	if poll.IsClosed {
		c.JSON(http.StatusConflict, gin.H{"error": "poll is closed"})
		return
	}
	if req.OptionIdx < 0 || req.OptionIdx >= len(poll.Options) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "optionIdx out of range"})
		return
	}

	added, err := h.Redis.SAdd(ctx, votersKey(pollID), req.VoterID).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not record vote"})
		return
	}
	if added == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "you have already voted on this poll"})
		return
	}

	newCount, err := h.Redis.HIncrBy(ctx, countsKey(pollID), strconv.Itoa(req.OptionIdx), 1).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not record vote"})
		return
	}

	// Durable audit trail. If this fails we don't roll back the live
	// counter — the vote already happened from the audience's point of
	// view — but we do log it loudly since it means Mongo and Redis have
	// drifted apart until the next cold-cache rebuild.
	vote := models.Vote{
		ID:        uuid.NewString(),
		PollID:    pollID,
		VoterID:   req.VoterID,
		OptionIdx: req.OptionIdx,
		CreatedAt: time.Now().UTC(),
	}
	if _, err := h.DB.Collection("votes").InsertOne(ctx, vote); err != nil {
		c.Error(err)
	}

	results, _ := h.getResults(ctx, poll)
	_ = newCount

	// Publish so every connected websocket client sees this instantly —
	// this is what makes the update "live" instead of poll-on-refresh.
	if err := h.Redis.Publish(ctx, ws.PollChannel(pollID), ws.MustJSON(results)).Err(); err != nil {
		c.Error(err)
	}

	c.JSON(http.StatusOK, results)
}

func (h *PollHandler) ClosePoll(c *gin.Context) {
	pollID := c.Param("id")
	userID := c.GetString("userId")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := h.DB.Collection("polls").UpdateOne(ctx,
		bson.M{"_id": pollID, "creatorId": userID},
		bson.M{"$set": bson.M{"isClosed": true}},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not close poll"})
		return
	}
	if res.MatchedCount == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "poll not found or you don't own it"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "closed"})
}

// WatchResults upgrades to a websocket and streams every future update for
// this poll. It sends one snapshot immediately so the UI has numbers before
// the first live vote arrives.
func (h *PollHandler) WatchResults(c *gin.Context) {
	pollID := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var poll models.Poll
	if err := h.DB.Collection("polls").FindOne(ctx, bson.M{"_id": pollID}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}

	results, _ := h.getResults(ctx, poll)
	h.Hub.SubscribeAndForward(context.Background(), pollID)
	h.Hub.ServeWS(c, pollID, ws.MustJSON(results))
}
