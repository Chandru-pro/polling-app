import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api.js'

export default function MyPolls() {
  const [polls, setPolls] = useState(null)
  const [error, setError] = useState('')

  useEffect(() => {
    api.myPolls().then(setPolls).catch((err) => setError(err.message))
  }, [])

  async function handleClose(id) {
    try {
      await api.closePoll(id)
      setPolls((prev) => prev.map((p) => (p.id === id ? { ...p, isClosed: true } : p)))
    } catch (err) {
      setError(err.message)
    }
  }

  if (error) return <div className="card"><p className="error">{error}</p></div>
  if (!polls) return <div className="card"><p className="muted">Loading…</p></div>

  return (
    <div className="card">
      <h2>My polls</h2>
      {polls.length === 0 && <p className="muted">You haven't created a poll yet. <Link to="/create">Create one</Link>.</p>}
      <ul className="poll-list">
        {polls.map((p) => (
          <li key={p.id} className="poll-list-item">
            <div>
              <Link to={`/poll/${p.id}`}>{p.question}</Link>
              <span className={`badge ${p.isClosed ? 'closed' : 'open'}`}>{p.isClosed ? 'Closed' : 'Live'}</span>
            </div>
            {!p.isClosed && (
              <button className="btn ghost small" onClick={() => handleClose(p.id)}>Close poll</button>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}
