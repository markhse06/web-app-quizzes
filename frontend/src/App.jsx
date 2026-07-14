import { useEffect, useState } from 'react'
import { Link, Navigate, Route, Routes, useNavigate } from 'react-router-dom'
import { ApiError, apiRequest, authApi, clearTokens, hasAccessToken, saveTokens } from './api/client'
import './App.css'

function AuthForm({ mode }) {
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const isLogin = mode === 'login'

  async function submit(event) {
    event.preventDefault()
    setError('')
    try {
      if (isLogin) {
        saveTokens(await authApi.login(email, password))
        navigate('/dashboard', { replace: true })
      } else {
        await authApi.register(email, password)
        navigate('/login', { replace: true, state: { registered: true } })
      }
    } catch (requestError) {
      setError(requestError.message)
    }
  }

  return <main className="auth-page">
    <section className="auth-card">
      <p className="eyebrow">Quiz host</p>
      <h1>{isLogin ? 'Вход для организатора' : 'Создайте аккаунт'}</h1>
      <p className="muted">{isLogin ? 'Управляйте квизами и игровыми сессиями.' : 'Зарегистрируйтесь, чтобы создавать квизы.'}</p>
      <form onSubmit={submit} className="auth-form">
        <label>Почта<input type="email" value={email} onChange={(event) => setEmail(event.target.value)} required /></label>
        <label>Пароль<input type="password" value={password} onChange={(event) => setPassword(event.target.value)} minLength="6" required /></label>
        {error && <p className="form-error" role="alert">{error}</p>}
        <button type="submit">{isLogin ? 'Войти' : 'Зарегистрироваться'}</button>
      </form>
      <p className="muted">{isLogin ? <>Нет аккаунта? <Link to="/register">Регистрация</Link></> : <>Уже зарегистрированы? <Link to="/login">Войти</Link></>}</p>
    </section>
  </main>
}

function ProtectedRoute({ children }) {
  return hasAccessToken() ? children : <Navigate to="/login" replace />
}

function Dashboard() {
  const navigate = useNavigate()
  const [quizzes, setQuizzes] = useState([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    apiRequest('/api/quizzes')
      .then(setQuizzes)
      .catch((requestError) => {
        setError(requestError.message)
        if (requestError instanceof ApiError && requestError.status === 401) navigate('/login', { replace: true })
      })
      .finally(() => setLoading(false))
  }, [navigate])

  return <main className="dashboard">
    <header className="dashboard-header">
      <div><p className="eyebrow">Quiz host</p><h1>Мои квизы</h1></div>
      <div className="header-actions"><Link className="button" to="/quizzes/new">Создать квиз</Link><button className="text-button" onClick={() => { clearTokens(); navigate('/login') }}>Выйти</button></div>
    </header>
    {loading && <p className="muted">Загружаем квизы…</p>}
    {error && <p className="form-error" role="alert">{error}</p>}
    {!loading && !error && <section className="quiz-grid">
      {quizzes.length === 0 ? <p className="empty-state">Квизов пока нет. Создайте первый, чтобы начать игру.</p> : quizzes.map((quiz) => <article className="quiz-card" key={quiz.ID}><h2>{quiz.Title}</h2><p>{quiz.QuestionDuration} сек. на вопрос</p><Link to={`/quizzes/${quiz.ID}`}>Открыть</Link></article>)}
    </section>}
    <section className="history"><h2>История игр</h2><p className="muted">Завершённые игры появятся здесь после запуска игровых сессий.</p></section>
  </main>
}

function Placeholder() {
  return <main className="auth-page"><section className="auth-card"><h1>Конструктор квиза</h1><p className="muted">Этот экран будет добавлен в следующей задаче.</p><Link to="/dashboard">Вернуться в кабинет</Link></section></main>
}

function App() {
  return <Routes>
    <Route path="/register" element={<AuthForm mode="register" />} />
    <Route path="/login" element={<AuthForm mode="login" />} />
    <Route path="/dashboard" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
    <Route path="/quizzes/new" element={<ProtectedRoute><Placeholder /></ProtectedRoute>} />
    <Route path="*" element={<Navigate to={hasAccessToken() ? '/dashboard' : '/login'} replace />} />
  </Routes>
}

export default App
