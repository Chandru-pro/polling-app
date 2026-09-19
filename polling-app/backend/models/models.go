package models

import "time"

type User struct {
	ID           string    `bson:"_id" json:"id"`
	Username     string    `bson:"username" json:"username"`
	PasswordHash string    `bson:"passwordHash" json:"-"`
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
}

type Option struct {
	Index int    `bson:"index" json:"index"`
	Text  string `bson:"text" json:"text"`
}

type Poll struct {
	ID        string    `bson:"_id" json:"id"`
	Question  string    `bson:"question" json:"question"`
	Options   []Option  `bson:"options" json:"options"`
	CreatorID string    `bson:"creatorId" json:"creatorId"`
	IsClosed  bool      `bson:"isClosed" json:"isClosed"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}

// Vote is the durable audit record. Redis holds the live counters;
// this is what we fall back to / reconcile against if Redis is cold.
type Vote struct {
	ID        string    `bson:"_id" json:"id"`
	PollID    string    `bson:"pollId" json:"pollId"`
	VoterID   string    `bson:"voterId" json:"voterId"`
	OptionIdx int       `bson:"optionIdx" json:"optionIdx"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}

// PollResults is what gets broadcast over the websocket and returned by the API.
type PollResults struct {
	PollID  string `json:"pollId"`
	Counts  []int  `json:"counts"`  // counts[i] = votes for Options[i]
	Total   int    `json:"total"`
}
