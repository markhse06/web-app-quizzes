import { useEffect, useRef, useState } from 'react'
import { Link, Navigate, Route, Routes, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { ApiError, apiRequest, authApi, clearTokens, getAccessToken, getApiUrl, hasAccessToken, saveTokens } from './api/client'
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
  const [launchingQuizID, setLaunchingQuizID] = useState('')

  useEffect(() => {
    apiRequest('/api/quizzes')
      .then(setQuizzes)
      .catch((requestError) => {
        setError(requestError.message)
        if (requestError instanceof ApiError && requestError.status === 401) navigate('/login', { replace: true })
      })
      .finally(() => setLoading(false))
  }, [navigate])

  async function launchGame(quizID) {
    setError('')
    setLaunchingQuizID(quizID)
    try {
      const session = await apiRequest('/api/sessions', { method: 'POST', body: JSON.stringify({ quiz_id: quizID }) })
      navigate(`/host/lobby/${session.pin_code}`)
    } catch (requestError) {
      setError(requestError.message)
    } finally {
      setLaunchingQuizID('')
    }
  }

  return <main className="dashboard">
    <header className="dashboard-header">
      <div><p className="eyebrow">Quiz host</p><h1>Мои квизы</h1></div>
      <div className="header-actions"><Link className="button" to="/quizzes/create">Создать квиз</Link><button className="text-button" onClick={() => { clearTokens(); navigate('/login') }}>Выйти</button></div>
    </header>
    {loading && <p className="muted">Загружаем квизы…</p>}
    {error && <p className="form-error" role="alert">{error}</p>}
    {!loading && !error && <section className="quiz-grid">
      {quizzes.length === 0 ? <p className="empty-state">Квизов пока нет. Создайте первый, чтобы начать игру.</p> : quizzes.map((quiz) => <article className="quiz-card" key={quiz.ID}><h2>{quiz.Title}</h2><p>{quiz.QuestionDuration} сек. на вопрос</p><div className="card-actions"><Link to={`/quizzes/${quiz.ID}`}>Открыть</Link><button onClick={() => launchGame(quiz.ID)} disabled={launchingQuizID === quiz.ID}>{launchingQuizID === quiz.ID ? 'Запускаем…' : 'Запустить игру'}</button></div></article>)}
    </section>}
    <section className="history"><h2>История игр</h2><p className="muted">Завершённые игры появятся здесь после запуска игровых сессий.</p></section>
  </main>
}

const newAnswer = () => ({ content: '', is_correct: false })
const newQuestion = () => ({ content: '', type: 'single', image_path: '', answers: [newAnswer(), newAnswer()] })

function QuizBuilder() {
  const navigate = useNavigate()
  const [title, setTitle] = useState('')
  const [duration, setDuration] = useState(20)
  const [questions, setQuestions] = useState([newQuestion()])
  const [error, setError] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [uploadingIndex, setUploadingIndex] = useState(null)

  function updateQuestion(questionIndex, update) {
    setQuestions((current) => current.map((question, index) => index === questionIndex ? { ...question, ...update } : question))
  }

  function updateAnswer(questionIndex, answerIndex, update) {
    setQuestions((current) => current.map((question, index) => {
      if (index !== questionIndex) return question
      const answers = question.answers.map((answer, currentAnswerIndex) => {
        if (currentAnswerIndex !== answerIndex) return answer
        return { ...answer, ...update }
      })
      return { ...question, answers }
    }))
  }

  function toggleCorrect(questionIndex, answerIndex) {
    setQuestions((current) => current.map((question, index) => {
      if (index !== questionIndex) return question
      const answers = question.answers.map((answer, currentAnswerIndex) => ({
        ...answer,
        is_correct: question.type === 'single'
          ? currentAnswerIndex === answerIndex ? !answer.is_correct : false
          : currentAnswerIndex === answerIndex ? !answer.is_correct : answer.is_correct,
      }))
      return { ...question, answers }
    }))
  }

  function changeQuestionType(questionIndex, type) {
    setQuestions((current) => current.map((question, index) => {
      if (index !== questionIndex) return question
      if (type === 'multiple') return { ...question, type }
      const firstCorrectIndex = question.answers.findIndex((answer) => answer.is_correct)
      return {
        ...question,
        type,
        answers: question.answers.map((answer, answerIndex) => ({ ...answer, is_correct: answerIndex === firstCorrectIndex })),
      }
    }))
  }

  async function uploadImage(questionIndex, file) {
    if (!file) return
    setError('')
    setUploadingIndex(questionIndex)
    try {
      const formData = new FormData()
      formData.append('file', file)
      const { path } = await apiRequest('/api/upload', { method: 'POST', body: formData })
      updateQuestion(questionIndex, { image_path: path })
    } catch (requestError) {
      setError(requestError.message)
    } finally {
      setUploadingIndex(null)
    }
  }

  function validate() {
    if (!title.trim()) return 'Укажите название квиза.'
    if (!Number.isInteger(Number(duration)) || Number(duration) < 1) return 'Время на вопрос должно быть положительным числом.'
    for (const question of questions) {
      if (!question.content.trim()) return 'Заполните текст каждого вопроса.'
      if (question.answers.some((answer) => !answer.content.trim())) return 'Заполните все варианты ответа.'
      if (!question.answers.some((answer) => answer.is_correct)) return 'В каждом вопросе должен быть правильный вариант.'
    }
    return ''
  }

  async function saveQuiz(event) {
    event.preventDefault()
    const validationError = validate()
    if (validationError) { setError(validationError); return }
    setIsSaving(true)
    setError('')
    try {
      await apiRequest('/api/quizzes', {
        method: 'POST',
        body: JSON.stringify({ title: title.trim(), question_duration: Number(duration), questions }),
      })
      navigate('/dashboard', { replace: true })
    } catch (requestError) {
      setError(requestError.message)
    } finally {
      setIsSaving(false)
    }
  }

  return <main className="builder-page">
    <header className="builder-header"><div><p className="eyebrow">Quiz host</p><h1>Конструктор квиза</h1></div><Link to="/dashboard">Отмена</Link></header>
    <form className="builder-form" onSubmit={saveQuiz}>
      <section className="builder-section"><label>Название квиза<input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Например, Космический квиз" /></label><label>Время на вопрос (сек)<input type="number" min="1" value={duration} onChange={(event) => setDuration(event.target.value)} /></label></section>
      {questions.map((question, questionIndex) => <section className="question-editor" key={questionIndex}>
        <div className="question-heading"><h2>Вопрос {questionIndex + 1}</h2>{questions.length > 1 && <button type="button" className="text-button danger" onClick={() => setQuestions((current) => current.filter((_, index) => index !== questionIndex))}>Удалить</button>}</div>
        <label>Текст вопроса<textarea value={question.content} onChange={(event) => updateQuestion(questionIndex, { content: event.target.value })} rows="3" /></label>
        <label>Тип вопроса<select value={question.type} onChange={(event) => changeQuestionType(questionIndex, event.target.value)}><option value="single">Одиночный выбор</option><option value="multiple">Множественный выбор</option></select></label>
        <label>Картинка<input type="file" accept="image/png,image/jpeg,image/gif,image/webp" onChange={(event) => uploadImage(questionIndex, event.target.files?.[0])} /></label>
        {uploadingIndex === questionIndex && <p className="muted">Загружаем картинку…</p>}
        {question.image_path && <img className="question-image" src={`${getApiUrl()}${question.image_path}`} alt="Предпросмотр вопроса" />}
        <div className="answers"><h3>Варианты ответа</h3>{question.answers.map((answer, answerIndex) => <div className="answer-row" key={answerIndex}><input value={answer.content} onChange={(event) => updateAnswer(questionIndex, answerIndex, { content: event.target.value })} placeholder={`Вариант ${answerIndex + 1}`} /><label className="correct"><input type="checkbox" checked={answer.is_correct} onChange={() => toggleCorrect(questionIndex, answerIndex)} />Правильный</label>{question.answers.length > 2 && <button type="button" className="remove-answer" onClick={() => updateQuestion(questionIndex, { answers: question.answers.filter((_, index) => index !== answerIndex) })}>×</button>}</div>)}
          {question.answers.length < 6 && <button type="button" className="secondary-button" onClick={() => updateQuestion(questionIndex, { answers: [...question.answers, newAnswer()] })}>Добавить вариант ответа</button>}</div>
      </section>)}
      {error && <p className="form-error" role="alert">{error}</p>}
      <div className="builder-actions"><button type="button" className="secondary-button" onClick={() => setQuestions((current) => [...current, newQuestion()])}>Добавить вопрос</button><button type="submit" disabled={isSaving || uploadingIndex !== null}>{isSaving ? 'Сохраняем…' : 'Сохранить квиз'}</button></div>
    </form>
  </main>
}

function HostRoom() {
  const { pin } = useParams()
  const navigate = useNavigate()
  const socketRef = useRef(null)
  const [players, setPlayers] = useState([])
  const [phase, setPhase] = useState('lobby')
  const [question, setQuestion] = useState(null)
  const [remaining, setRemaining] = useState(0)
  const [answerCounts, setAnswerCounts] = useState({})
  const [leaderboard, setLeaderboard] = useState([])
  const [connectionError, setConnectionError] = useState('')

  useEffect(() => {
    const apiURL = getApiUrl()
    const url = `${apiURL.replace(/^http/, 'ws')}/ws/join?pin=${encodeURIComponent(pin)}&name=host`
    const socket = new WebSocket(url, ['access_token', getAccessToken()])
    socketRef.current = socket
    socket.onmessage = ({ data }) => {
      const message = JSON.parse(data)
      if (message.type === 'lobby_update') setPlayers(message.players)
      if (message.type === 'question') {
        setQuestion(message.question)
        setRemaining(message.duration_seconds)
        setPhase('question')
      }
      if (message.type === 'question_result') {
        setAnswerCounts(message.answer_counts ?? {})
        setLeaderboard(message.leaderboard ?? [])
        setPhase('result')
      }
      if (message.type === 'game_finished') setPhase('finished')
    }
    socket.onerror = () => setConnectionError('Не удалось подключиться к игровой комнате.')
    socket.onclose = () => { socketRef.current = null }
    return () => socket.close()
  }, [pin])

  useEffect(() => {
    if (phase !== 'question') return undefined
    const timer = window.setInterval(() => setRemaining((value) => Math.max(value - 1, 0)), 1000)
    return () => window.clearInterval(timer)
  }, [phase, question?.id])

  function send(type) {
    if (socketRef.current?.readyState === WebSocket.OPEN) socketRef.current.send(JSON.stringify({ type }))
  }

  if (connectionError) return <main className="host-room"><p className="form-error">{connectionError}</p><Link to="/dashboard">Вернуться в кабинет</Link></main>
  if (phase === 'lobby') return <main className="host-room lobby"><p className="eyebrow">Игровая комната</p><h1>PIN-код <strong>{pin}</strong></h1><p className="muted">Игроки входят по этому коду.</p><section className="players-panel"><h2>В лобби — {players.length}</h2>{players.length ? <ul>{players.map((player) => <li key={player.id}>{player.name}</li>)}</ul> : <p className="muted">Ожидаем игроков…</p>}</section><button onClick={() => send('start_game')}>Начать игру</button></main>
  if (phase === 'question' && question) return <main className="host-room"><header className="game-header"><span>Вопрос</span><strong className="timer">{remaining} c</strong></header><section className="host-question"><h1>{question.content}</h1>{question.image_path && <img src={`${getApiUrl()}${question.image_path}`} alt="К вопросу" />}<div className="host-answers">{question.answers.map((answer, index) => <div key={answer.id}><span>{index + 1}</span>{answer.content}</div>)}</div></section><button onClick={() => send('question_time_over')}>Завершить вопрос</button></main>
  if (phase === 'result' && question) return <ResultScreen question={question} answerCounts={answerCounts} leaderboard={leaderboard} onNext={() => send('next_question')} />
  return <main className="host-room"><p className="eyebrow">Игра завершена</p><h1>Спасибо за игру!</h1><Leaderboard players={leaderboard} /><button onClick={() => navigate('/dashboard')}>Вернуться в кабинет</button></main>
}

function Leaderboard({ players }) {
  return <section className="leaderboard"><h2>Лидерборд</h2><ol>{players.slice(0, 5).map((player) => <li key={player.id}><span>{player.name}</span><strong>{player.score}</strong></li>)}</ol></section>
}

function ResultScreen({ question, answerCounts, leaderboard, onNext }) {
  const maxCount = Math.max(1, ...Object.values(answerCounts))
  return <main className="host-room"><p className="eyebrow">Результаты вопроса</p><h1>{question.content}</h1><section className="result-layout"><div className="answer-chart"><h2>Ответы игроков</h2>{question.answers.map((answer) => <div className="chart-row" key={answer.id}><div><span>{answer.content}</span><strong>{answerCounts[answer.id] ?? 0}</strong></div><div className="bar-track"><div className="bar" style={{ width: `${((answerCounts[answer.id] ?? 0) / maxCount) * 100}%` }} /></div></div>)}</div><Leaderboard players={leaderboard} /></section><button onClick={onNext}>Следующий вопрос</button></main>
}

function JoinGame() {
  const navigate = useNavigate()
  const [pin, setPin] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState('')

  function join(event) {
    event.preventDefault()
    const trimmedName = name.trim()
    if (!/^\d{6}$/.test(pin)) { setError('PIN-код должен состоять из 6 цифр.'); return }
    if (!trimmedName) { setError('Введите своё имя.'); return }
    if (trimmedName.toLowerCase() === 'host') { setError('Это имя зарезервировано для организатора.'); return }
    navigate(`/play/${pin}?name=${encodeURIComponent(trimmedName)}`)
  }

  return <main className="player-page">
    <section className="player-card join-card">
      <p className="eyebrow">Quiz</p><h1>Войти в игру</h1><p className="muted">Введите PIN-код комнаты и своё имя.</p>
      <form onSubmit={join} className="player-form"><label>PIN-код<input inputMode="numeric" maxLength="6" value={pin} onChange={(event) => setPin(event.target.value.replace(/\D/g, ''))} placeholder="123456" required /></label><label>Твоё имя<input value={name} onChange={(event) => setName(event.target.value)} maxLength="40" placeholder="Например, Маша" required /></label>{error && <p className="form-error" role="alert">{error}</p>}<button type="submit">Войти</button></form>
    </section>
  </main>
}

function PlayerRoom() {
  const { pin } = useParams()
  const [searchParams] = useSearchParams()
  const name = searchParams.get('name') ?? ''
  const navigate = useNavigate()
  const socketRef = useRef(null)
  const playerRef = useRef(null)
  const selectedAnswerIDsRef = useRef([])
  const leaderboardRef = useRef([])
  const [phase, setPhase] = useState('waiting')
  const [player, setPlayer] = useState(null)
  const [question, setQuestion] = useState(null)
  const [selectedAnswerIDs, setSelectedAnswerIDs] = useState([])
  const [submitted, setSubmitted] = useState(false)
  const [feedback, setFeedback] = useState(null)
  const [finalResult, setFinalResult] = useState(null)
  const [connectionError, setConnectionError] = useState('')

  useEffect(() => {
    if (!name) { navigate('/', { replace: true }); return undefined }
    const apiURL = getApiUrl()
    const url = `${apiURL.replace(/^http/, 'ws')}/ws/join?pin=${encodeURIComponent(pin)}&name=${encodeURIComponent(name)}`
    const socket = new WebSocket(url)
    socketRef.current = socket
    socket.onmessage = ({ data }) => {
      const message = JSON.parse(data)
      if (message.type === 'joined') {
        playerRef.current = message.participant
        setPlayer(message.participant)
      }
      if (message.type === 'question') {
        setQuestion(message.question)
        selectedAnswerIDsRef.current = []
        setSelectedAnswerIDs([])
        setSubmitted(false)
        setPhase('answers')
      }
      if (message.type === 'question_result') {
        const selected = new Set(selectedAnswerIDsRef.current)
        const correct = new Set(message.correct_answer_ids ?? [])
        const isCorrect = selected.size === correct.size && [...correct].every((answerID) => selected.has(answerID))
        leaderboardRef.current = message.leaderboard ?? []
        const me = leaderboardRef.current.find((candidate) => candidate.id === playerRef.current?.id)
        setFeedback({ isCorrect, score: me?.score ?? playerRef.current?.score ?? 0 })
        if (me) {
          playerRef.current = me
          setPlayer(me)
        }
        setPhase('feedback')
      }
      if (message.type === 'game_finished') {
        const place = leaderboardRef.current.findIndex((candidate) => candidate.id === playerRef.current?.id) + 1
        const me = leaderboardRef.current.find((candidate) => candidate.id === playerRef.current?.id) ?? playerRef.current
        setFinalResult({ place, score: me?.score ?? 0 })
        setPhase('finished')
      }
    }
    socket.onerror = () => setConnectionError('Не удалось подключиться к комнате. Проверьте PIN-код и повторите попытку.')
    socket.onclose = () => { socketRef.current = null }
    return () => socket.close()
  }, [name, navigate, pin])

  function submit(answerIDs) {
    if (socketRef.current?.readyState !== WebSocket.OPEN || submitted) return
    socketRef.current.send(JSON.stringify({ type: 'submit_answer', answer_ids: answerIDs }))
    setSubmitted(true)
  }

  function chooseAnswer(answerID) {
    if (!question || submitted) return
    if (question.type === 'single') {
      selectedAnswerIDsRef.current = [answerID]
      setSelectedAnswerIDs([answerID])
      submit([answerID])
      return
    }
    setSelectedAnswerIDs((current) => {
      const next = current.includes(answerID) ? current.filter((id) => id !== answerID) : [...current, answerID]
      selectedAnswerIDsRef.current = next
      return next
    })
  }

  if (connectionError) return <main className="player-page"><section className="player-card"><p className="form-error">{connectionError}</p><Link to="/">Вернуться ко входу</Link></section></main>
  if (phase === 'waiting') return <main className="player-page"><section className="player-card waiting-card"><p className="eyebrow">Ты в игре!</p><h1>Ждём начала…</h1><p className="player-name">{name}</p><p className="muted">Организатор скоро запустит первый вопрос.</p></section></main>
  if (phase === 'answers' && question) return <main className="player-page answer-page"><section className="player-question"><p className="eyebrow">Вопрос</p><h1>{question.content}</h1>{question.image_path && <img src={`${getApiUrl()}${question.image_path}`} alt="Иллюстрация вопроса" />}</section><div className="answer-buttons">{question.answers.map((answer, index) => <button className={`answer-button answer-${index % 4} ${selectedAnswerIDs.includes(answer.id) ? 'selected' : ''}`} key={answer.id} disabled={submitted} onClick={() => chooseAnswer(answer.id)}><span>{index + 1}</span>{answer.content}</button>)}</div>{question.type === 'multiple' && <button className="submit-answer" disabled={submitted || selectedAnswerIDs.length === 0} onClick={() => submit(selectedAnswerIDs)}>{submitted ? 'Ответ отправлен' : 'Подтвердить ответ'}</button>}{question.type === 'single' && submitted && <p className="answer-sent">Ответ отправлен</p>}</main>
  if (phase === 'feedback' && feedback) return <main className="player-page"><section className={`player-card feedback-card ${feedback.isCorrect ? 'correct-feedback' : 'wrong-feedback'}`}><p className="eyebrow">Результат</p><h1>{feedback.isCorrect ? 'Правильно! +1 балл' : 'Неправильно'}</h1><p>Текущий счёт: <strong>{feedback.score}</strong></p><p className="muted">Ждём следующий вопрос…</p></section></main>
  return <main className="player-page"><section className="player-card final-card"><p className="eyebrow">Игра завершена</p><h1>Спасибо за игру!</h1><p className="final-place">{finalResult?.place ? `${finalResult.place} место` : 'Результат готов'}</p><p>Твой счёт: <strong>{finalResult?.score ?? player?.score ?? 0}</strong></p><Link className="button" to="/">Играть ещё</Link></section></main>
}

function App() {
  return <Routes>
    <Route path="/" element={<JoinGame />} />
    <Route path="/join" element={<JoinGame />} />
    <Route path="/play/:pin" element={<PlayerRoom />} />
    <Route path="/register" element={<AuthForm mode="register" />} />
    <Route path="/login" element={<AuthForm mode="login" />} />
    <Route path="/dashboard" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
    <Route path="/quizzes/create" element={<ProtectedRoute><QuizBuilder /></ProtectedRoute>} />
    <Route path="/host/lobby/:pin" element={<ProtectedRoute><HostRoom /></ProtectedRoute>} />
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes>
}

export default App
