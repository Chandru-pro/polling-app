import { BrowserRouter, Routes, Route, Link, Navigate } from 'react-router-dom'
import { useState, useEffect } from 'react'
import Login from './pages/Login.jsx'
import Signup from './pages/Signup.jsx'
import CreatePoll from './pages/CreatePoll.jsx'
import PollView from './pages/PollView.jsx'
import MyPolls from './pages/MyPolls.jsx'
import Home from './pages/Home.jsx'

function useAuth() {
  const [username, setUsername] = useState(localStorage.getItem('username'))
  useEffect(() => {
    const onStorage = () => setUsername(localStorage.getItem('username'))
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])
  return {
    username,
    login: (u) => { localStorage.setItem('username', u); setUsername(u) },
    logout: () => {
      localStorage.removeItem('token')
      localStorage.removeItem('username')
      setUsername(null)
    },
  }
}

function RequireAuth({ username, children }) {
  if (!username) return <Navigate to="/login" replace />
  return children
}

export default function App() {
  const auth = useAuth()

  return (
    <BrowserRouter>
      <div className="app-shell">
        <header className="nav">
          <Link to="/" className="brand">LivePoll</Link>
          <nav>
            {auth.username ? (
              <>
                <Link to="/my-polls">My polls</Link>
                <Link to="/create">New poll</Link>
                <span className="nav-user">Hi, {auth.username}</span>
                <button className="link-btn" onClick={auth.logout}>Log out</button>
              </>
            ) : (
              <>
                <Link to="/login">Log in</Link>
                <Link to="/signup">Sign up</Link>
              </>
            )}
          </nav>
        </header>

        <main className="content">
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/login" element={<Login onLogin={auth.login} />} />
            <Route path="/signup" element={<Signup onLogin={auth.login} />} />
            <Route
              path="/create"
              element={<RequireAuth username={auth.username}><CreatePoll /></RequireAuth>}
            />
            <Route
              path="/my-polls"
              element={<RequireAuth username={auth.username}><MyPolls /></RequireAuth>}
            />
            <Route path="/poll/:id" element={<PollView />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  )
}
