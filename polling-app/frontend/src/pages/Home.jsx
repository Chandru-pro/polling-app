import { Link } from 'react-router-dom'

export default function Home() {
  return (
    <div className="hero">
      <h1>Create a poll. Share a link. Watch votes land live.</h1>
      <p>No refreshing. No waiting. Just a question, an audience, and results that update the second someone taps an option.</p>
      <div className="hero-actions">
        <Link to="/signup" className="btn primary">Get started</Link>
        <Link to="/login" className="btn ghost">I already have an account</Link>
      </div>
    </div>
  )
}
