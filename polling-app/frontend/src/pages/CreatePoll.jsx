import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api.js'

export default function CreatePoll() {
  const [question, setQuestion] = useState('')
  const [options, setOptions] = useState(['', ''])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  function updateOption(i, value) {
    setOptions((prev) => prev.map((o, idx) => (idx === i ? value : o)))
  }

  function addOption() {
    if (options.length >= 10) return
    setOptions((prev) => [...prev, ''])
  }

  function removeOption(i) {
    if (options.length <= 2) return
    setOptions((prev) => prev.filter((_, idx) => idx !== i))
  }

  async function handleSubmit(e) {
    e.preventDefault()
    setError('')
    const cleaned = options.map((o) => o.trim()).filter(Boolean)
    if (cleaned.length < 2) {
      setError('Add at least two options.')
      return
    }
    setLoading(true)
    try {
      const poll = await api.createPoll(question.trim(), cleaned)
      navigate(`/poll/${poll.id}`)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="card">
      <h2>New poll</h2>
      <form onSubmit={handleSubmit}>
        <label>Question
          <input value={question} onChange={(e) => setQuestion(e.target.value)} maxLength={300} required placeholder="What should we ask?" />
        </label>

        <div className="options-list">
          {options.map((opt, i) => (
            <div className="option-row" key={i}>
              <input
                value={opt}
                onChange={(e) => updateOption(i, e.target.value)}
                placeholder={`Option ${i + 1}`}
                maxLength={120}
              />
              {options.length > 2 && (
                <button type="button" className="icon-btn" onClick={() => removeOption(i)} aria-label="Remove option">✕</button>
              )}
            </div>
          ))}
        </div>

        {options.length < 10 && (
          <button type="button" className="btn ghost small" onClick={addOption}>+ Add option</button>
        )}

        {error && <p className="error">{error}</p>}
        <button className="btn primary" type="submit" disabled={loading}>
          {loading ? 'Creating…' : 'Create poll'}
        </button>
      </form>
    </div>
  )
}
