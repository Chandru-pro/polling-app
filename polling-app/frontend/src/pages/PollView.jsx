import { useEffect, useRef, useState, useCallback } from 'react'
import { useParams } from 'react-router-dom'
import { api, getOrCreateVoterId } from '../lib/api.js'

function ResultsBars({ poll, results }) {
  const total = results?.total ?? 0
  return (
    <div className="results">
      {poll.options.map((opt, i) => {
        const count = results?.counts?.[i] ?? 0
        const pct = total > 0 ? Math.round((count / total) * 100) : 0
        return (
          <div className="result-row" key={opt.index}>
            <div className="result-label">
              <span>{opt.text}</span>
              <span className="result-meta">{count} vote{count === 1 ? '' : 's'} · {pct}%</span>
            </div>
            <div className="bar-track">
              <div className="bar-fill" style={{ width: `${pct}%` }} />
            </div>
          </div>
        )
      })}
      <p className="muted total-votes">{total} total vote{total === 1 ? '' : 's'} · updating live</p>
    </div>
  )
}

export default function PollView() {
  const { id } = useParams()
  const [poll, setPoll] = useState(null)
  const [results, setResults] = useState(null)
  const [error, setError] = useState('')
  const [voting, setVoting] = useState(false)
  const [hasVoted, setHasVoted] = useState(() => localStorage.getItem(`voted:${id}`) === '1')
  const [connected, setConnected] = useState(false)
  const wsRef = useRef(null)

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const p = await api.getPoll(id)
        if (cancelled) return
        setPoll(p)
        const r = await api.getResults(id)
        if (cancelled) return
        setResults(r)
      } catch (err) {
        if (!cancelled) setError(err.message)
      }
    }
    load()
    return () => { cancelled = true }
  }, [id])

  // Live channel: one websocket per poll page, fed by the backend's Redis
  // pub/sub relay. Reconnects on drop so a flaky connection doesn't strand
  // the audience on stale numbers.
  useEffect(() => {
    let retryTimer
    let closedByUs = false

    function connect() {
      const socket = new WebSocket(api.wsUrl(id))
      wsRef.current = socket
      socket.onopen = () => setConnected(true)
      socket.onmessage = (event) => {
        try {
          setResults(JSON.parse(event.data))
        } catch { /* ignore malformed frame */ }
      }
      socket.onclose = () => {
        setConnected(false)
        if (!closedByUs) retryTimer = setTimeout(connect, 1500)
      }
      socket.onerror = () => socket.close()
    }
    connect()

    return () => {
      closedByUs = true
      clearTimeout(retryTimer)
      wsRef.current?.close()
    }
  }, [id])

  const handleVote = useCallback(async (optionIdx) => {
    setError('')
    setVoting(true)
    try {
      const r = await api.vote(id, optionIdx, getOrCreateVoterId())
      setResults(r)
      localStorage.setItem(`voted:${id}`, '1')
      setHasVoted(true)
    } catch (err) {
      if (err.message.includes('already voted')) {
        setHasVoted(true)
        localStorage.setItem(`voted:${id}`, '1')
      } else {
        setError(err.message)
      }
    } finally {
      setVoting(false)
    }
  }, [id])

  function copyLink() {
    navigator.clipboard.writeText(window.location.href)
  }

  if (error && !poll) return <div className="card"><p className="error">{error}</p></div>
  if (!poll) return <div className="card"><p className="muted">Loading poll…</p></div>

  return (
    <div className="card poll-card">
      <div className="poll-header">
        <h2>{poll.question}</h2>
        <button className="btn ghost small" onClick={copyLink}>Copy share link</button>
      </div>
      <p className={`live-indicator ${connected ? 'live' : 'offline'}`}>
        {connected ? '● Live' : '○ Reconnecting…'}
      </p>

      {poll.isClosed && <p className="muted">This poll is closed — showing final results.</p>}

      {!hasVoted && !poll.isClosed ? (
        <div className="vote-options">
          {poll.options.map((opt) => (
            <button key={opt.index} className="btn option-btn" disabled={voting} onClick={() => handleVote(opt.index)}>
              {opt.text}
            </button>
          ))}
        </div>
      ) : (
        <ResultsBars poll={poll} results={results} />
      )}

      {error && <p className="error">{error}</p>}

      {hasVoted && !poll.isClosed && (
        <p className="muted">Thanks for voting — results above update live as more votes come in.</p>
      )}
    </div>
  )
}
