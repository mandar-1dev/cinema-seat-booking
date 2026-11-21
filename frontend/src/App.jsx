import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

const foodMenu = [
  { id: 'popcorn', name: 'Classic Popcorn', detail: 'Large salted', price: 320, emoji: '🍿' },
  { id: 'nachos', name: 'Loaded Nachos', detail: 'Cheese & jalapeno', price: 280, emoji: '🧀' },
  { id: 'cola', name: 'Cold Drink', detail: '500 ml', price: 180, emoji: '🥤' },
  { id: 'combo', name: 'Movie Combo', detail: 'Popcorn + drink', price: 450, emoji: '🎬' },
]

function getDemoOwnerId() {
  let ownerId = localStorage.getItem('cinema-demo-owner')
  if (!ownerId) {
    ownerId = window.crypto.randomUUID()
    localStorage.setItem('cinema-demo-owner', ownerId)
  }
  return ownerId
}

function App() {
  const [movies, setMovies] = useState([])
  const [movie, setMovie] = useState(null)
  const [seats, setSeats] = useState([])
  const [session, setSession] = useState(null)
  const [message, setMessage] = useState('')
  const [page, setPage] = useState('book')
  const [authOpen, setAuthOpen] = useState(false)
  const [authMode, setAuthMode] = useState('login')
  const [authForm, setAuthForm] = useState({ full_name: '', email: '', phone: '', password: '' })
  const [authSubmitting, setAuthSubmitting] = useState(false)
  const [demoMode, setDemoMode] = useState(false)
  const [demoNameOpen, setDemoNameOpen] = useState(false)
  const [demoCustomerName, setDemoCustomerName] = useState('')
  const [demoNameInput, setDemoNameInput] = useState('')
  const [systemStatus, setSystemStatus] = useState(null)
  const [auth, setAuth] = useState(() => {
    try { return JSON.parse(localStorage.getItem('cinema-auth')) } catch { return null }
  })
  const authRef = useRef(auth)
  const [cart, setCart] = useState({})
  const [countdown, setCountdown] = useState(120)
  const [paymentOpen, setPaymentOpen] = useState(false)
  const [paymentProcessing, setPaymentProcessing] = useState(false)
  const [ticket, setTicket] = useState(null)
  const [ticketQrUrl, setTicketQrUrl] = useState('')
  const [ticketQrDataUrl, setTicketQrDataUrl] = useState('')
  const [pdfGenerating, setPdfGenerating] = useState(false)
  const [toast, setToast] = useState('')
  const [showDate] = useState(() => new Intl.DateTimeFormat('en-IN', { dateStyle: 'medium' }).format(new Date()))

  const loadSeats = useCallback(async (movieId) => {
    try {
      const response = await fetch(`/movies/${movieId}/seats`)
      if (response.ok) setSeats(await response.json())
    } catch {
      setMessage('Could not update seat availability')
    }
  }, [])

  useEffect(() => {
    fetch('/movies')
      .then((response) => response.ok ? response.json() : Promise.reject())
      .then((items) => {
        setMovies(items)
        setMovie(items[0] || null)
      })
      .catch(() => setMessage('Could not load movies'))
    fetch('/system/status')
      .then((response) => response.ok ? response.json() : null)
      .then((status) => setSystemStatus(status))
      .catch(() => setSystemStatus(null))
  }, [])

  useEffect(() => {
    if (!movie) return
    loadSeats(movie.id)
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const socket = new WebSocket(`${protocol}//${window.location.host}/ws/seats/${movie.id}`)
    socket.onmessage = () => loadSeats(movie.id)
    return () => socket.close()
  }, [movie, loadSeats])

  useEffect(() => {
    if (!session) return
    const timer = window.setInterval(() => {
      setCountdown((value) => {
        if (value <= 1) {
          window.clearInterval(timer)
          setSession(null)
          setMessage('Your seat hold expired')
          loadSeats(movie.id)
          return 0
        }

        return value - 1
      })
    }, 1000)
    return () => window.clearInterval(timer)
  }, [session, movie, loadSeats])

  useEffect(() => {
    if (!ticket?.id || (!ticket.demoOnly && !auth?.access_token)) return
    let objectUrl = ''
    const qrPath = ticket.demoOnly
      ? `/demo/tickets/${encodeURIComponent(ticket.id)}/qr`
      : `/tickets/${encodeURIComponent(ticket.id)}/qr`
    const qrRequest = ticket.demoOnly ? fetch(qrPath) : authenticatedFetch(qrPath)
    qrRequest
      .then((response) => response.ok ? response.blob() : Promise.reject())
      .then((blob) => {
        objectUrl = URL.createObjectURL(blob)
        setTicketQrUrl(objectUrl)
        const reader = new FileReader()
        reader.onload = () => setTicketQrDataUrl(reader.result)
        reader.onerror = () => setTicketQrDataUrl('')
        reader.readAsDataURL(blob)
      })
      .catch(() => {
        setTicketQrUrl('')
        setTicketQrDataUrl('')
      })

    return () => {
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [ticket, auth])

  useEffect(() => {
    if (!toast) return
    const timer = window.setTimeout(() => setToast(''), 4500)
    return () => window.clearTimeout(timer)
  }, [toast])

  async function downloadTicketPDF() {
    if (!ticket || !ticketQrDataUrl) return
    setPdfGenerating(true)
    try {
      const { jsPDF } = await import('jspdf')
      const details = [
        ['CUSTOMER', ticket.customerName],
        ['CINEMA / SCREEN', `${ticket.cinema} / ${ticket.screen}`],
        ['SHOW DATE', ticket.showDate],
        ['SHOWTIME', ticket.showTime],
        ['SEAT', ticket.seat],
        ['FOOD', ticket.foodItems],
        [ticket.demoOnly ? 'DEMO AMOUNT' : 'AMOUNT PAID', ticket.demoOnly ? `INR ${ticket.amount} (NO CHARGE)` : `INR ${ticket.amount}`],
        ['PAYMENT REF', ticket.paymentId],
        ['BOOKING REF', ticket.bookingId],
      ]
      if (!ticket.demoOnly) {
        details.splice(1, 0,
          ['EMAIL', ticket.customerEmail],
          ['PHONE', ticket.customerPhone || 'Not provided'],
        )
      }
      const pdf = new jsPDF({ orientation: 'portrait', unit: 'mm', format: [105, 210] })
      pdf.setFont('helvetica', 'normal')
      pdf.setFontSize(8)
      const preparedDetails = details.map(([label, value]) => ({
        label,
        lines: pdf.splitTextToSize(String(value || '-'), 50),
      }))
      const detailsEnd = 48 + preparedDetails.reduce((height, detail) => height + Math.max(6, detail.lines.length * 4), 0)
      const qrY = detailsEnd + 12
      pdf.setFillColor(16, 16, 20)
      pdf.rect(0, 0, 105, 210, 'F')
      pdf.setTextColor(244, 244, 245)
      pdf.setFont('helvetica', 'bold')
      pdf.setFontSize(18)
      pdf.text('CINEMORA', 10, 15)
      pdf.setTextColor(233, 75, 103)
      pdf.setFontSize(8)
      pdf.text(ticket.demoOnly ? 'DEMO ONLY - NOT VALID FOR ENTRY' : 'MOVIE ENTRY TICKET', 10, 21)
      pdf.setDrawColor(73, 73, 82)
      pdf.line(10, 26, 95, 26)
      pdf.setTextColor(245, 245, 247)
      pdf.setFont('helvetica', 'bold')
      pdf.setFontSize(13)
      pdf.text(ticket.movie, 10, 35, { maxWidth: 85 })
      pdf.setFont('helvetica', 'normal')
      pdf.setFontSize(8)
      let y = 44
      for (const detail of preparedDetails) {
        pdf.setTextColor(160, 160, 170)
        pdf.text(detail.label, 10, y)
        pdf.setTextColor(245, 245, 247)
        pdf.text(detail.lines, 45, y, { maxWidth: 50 })
        y += Math.max(6, detail.lines.length * 4)
      }
      pdf.setDrawColor(73, 73, 82)
      pdf.line(10, detailsEnd + 4, 95, detailsEnd + 4)
      pdf.setTextColor(170, 170, 180)
      pdf.setFontSize(6)
      pdf.text(`Ticket: ${ticket.id}`, 10, detailsEnd + 9, { maxWidth: 85 })
      pdf.addImage(ticketQrDataUrl, 'PNG', 34, qrY, 37, 37)
      pdf.text(`${ticket.demoOnly ? 'DEMO PASS - NOT VALID FOR ENTRY' : 'Issued'}: ${ticket.issuedAt}`, 10, qrY + 45, { maxWidth: 85 })
      pdf.save(`cinemora-ticket-${ticket.seat}.pdf`)
    } catch {
      setToast('Could not generate the ticket PDF. Please try again.')
    } finally {
      setPdfGenerating(false)
    }
  }

  async function holdSeat(seatId) {
    setMessage('')
    const userId = auth?.user?.id || getDemoOwnerId()
    const response = await fetch(`/movies/${movie.id}/seats/${seatId}/hold`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId }),
    })
    const body = await response.json()
    if (!response.ok) {
      setMessage(body.error || 'Could not hold this seat')
      return
    }
    setSession(body)
    setCountdown(120)
  }

  async function confirmSession() {
    if (demoMode) {
      setPaymentOpen(true)
      return
    }
    if (!auth?.access_token) {
      setAuthOpen(true)
      setMessage('Sign in to continue to payment')
      return
    }
    setPaymentOpen(true)
  }

  async function refreshAuthTokens() {
    const currentAuth = authRef.current
    if (!currentAuth?.refresh_token) {
      localStorage.removeItem('cinema-auth')
      authRef.current = null
      setAuth(null)
      return null
    }

    try {
      const response = await fetch('/auth/refresh', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: currentAuth.refresh_token }),
      })
      if (!response.ok) {
        localStorage.removeItem('cinema-auth')
        authRef.current = null
        setAuth(null)
        return null
      }

      const refreshedAuth = await response.json()
      localStorage.setItem('cinema-auth', JSON.stringify(refreshedAuth))
      authRef.current = refreshedAuth
      setAuth(refreshedAuth)
      return refreshedAuth
    } catch {
      setMessage('Could not refresh your sign-in. Please try again.')
      return null
    }
  }

  async function authenticatedFetch(url, options = {}) {
    const currentAuth = authRef.current
    if (!currentAuth?.access_token) {
      return fetch(url, options)
    }

    const send = (accessToken) => {
      const headers = new Headers(options.headers || {})
      headers.set('Authorization', `Bearer ${accessToken}`)
      return fetch(url, { ...options, headers })
    }

    const response = await send(currentAuth.access_token)
    if (response.status !== 401) return response

    const refreshedAuth = await refreshAuthTokens()
    if (!refreshedAuth) {
      setAuthOpen(true)
      setMessage('Your sign-in expired. Please sign in again to continue.')
      return response
    }
    return send(refreshedAuth.access_token)
  }

  async function payWithRazorpay() {
    if (!session || !authRef.current?.access_token) return
    setPaymentProcessing(true)
    setMessage('')

    let orderResponse
    let order
    try {
      orderResponse = await authenticatedFetch('/payments/orders', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          booking_id: session.session_id,
          amount: total,
          currency: 'INR',
        }),
      })
      order = await orderResponse.json()
    } catch {
      setPaymentProcessing(false)
      setToast('Razorpay service is currently unavailable. Please try again later.')
      return
    }
    if (!orderResponse.ok) {
      setPaymentProcessing(false)
      if (orderResponse.status === 401) {
        setPaymentOpen(false)
        setAuthOpen(true)
        setMessage('Your sign-in expired. Please sign in again to continue.')
        return
      }
      if (orderResponse.status === 502 || orderResponse.status === 503) {
        setToast('Razorpay service is currently unavailable. Please try again later.')
        return
      }
      setPaymentOpen(false)
      setMessage(order.error || 'Could not create a payment order')
      return
    }

    if (!window.Razorpay) {
      setPaymentProcessing(false)
      setPaymentOpen(false)
      setToast('Razorpay checkout could not be loaded. Please try again later.')
      return
    }

    const checkout = new window.Razorpay({
      key: order.key_id,
      amount: order.amount,
      currency: order.currency,
      order_id: order.order_id,
      name: 'Cinemora',
      description: `${movie.title} · Seat ${session.seat_id}`,
      handler: async (payment) => {
        const verifyResponse = await authenticatedFetch('/payments/verify', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            razorpay_order_id: payment.razorpay_order_id,
            razorpay_payment_id: payment.razorpay_payment_id,
            razorpay_signature: payment.razorpay_signature,
            session_id: session.session_id,
          }),
        })
        const verified = await verifyResponse.json()
        setPaymentProcessing(false)
        setPaymentOpen(false)
        if (!verifyResponse.ok) {
          setMessage(verified.error || 'Payment verification failed')
          return
        }
        setTicketQrUrl('')
        setTicketQrDataUrl('')
        setTicket({
          id: verified.ticket_id,
          bookingId: session.session_id,
          seat: session.seat_id,
          movie: movie.title,
          customerName: authRef.current?.user?.full_name || '',
          customerEmail: authRef.current?.user?.email || '',
          customerPhone: authRef.current?.user?.phone || '',
          amount: total,
          foodItems: foodMenu
            .filter((item) => cart[item.id] > 0)
            .map((item) => `${item.name} x${cart[item.id]}`)
            .join(', ') || 'None',
          showDate,
          showTime: '7:30 PM',
          cinema: 'Cinemora Central',
          screen: 'Audi 1',
          paymentId: payment.razorpay_payment_id,
          issuedAt: new Date().toLocaleString('en-IN'),
        })
        setSession(null)
        setPage('confirmation')
      },
      modal: {
        ondismiss: () => {
          setPaymentProcessing(false)
        },
      },
      theme: { color: '#e94b67' },
    })
    checkout.on('payment.failed', () => {
      setPaymentProcessing(false)
      setToast('Payment was not completed. Your seat remains held until the timer expires.')
    })
    checkout.open()
  }

  async function payWithDemo() {
    if (!session || !demoCustomerName) return
    setPaymentProcessing(true)
    setMessage('')
    try {
      const response = await fetch('/demo/checkout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          session_id: session.session_id,
          owner_id: authRef.current?.user?.id || getDemoOwnerId(),
          customer_name: demoCustomerName,
          movie_id: movie.id,
          show_date: showDate,
          food_items: foodMenu
            .filter((item) => cart[item.id] > 0)
            .map((item) => ({ id: item.id, quantity: cart[item.id] })),
        }),
      })
      const result = await response.json()
      if (!response.ok || !result.ticket?.id) {
        setMessage(result.error || 'Demo checkout could not be completed. Please try again.')
        return
      }
      const pass = result.ticket
      setTicketQrUrl('')
      setTicketQrDataUrl('')
      setTicket({
        id: pass.id,
        bookingId: pass.booking_id,
        seat: pass.seat_id,
        movie: pass.movie_title,
        customerName: pass.customer_name,
        amount: pass.amount,
        foodItems: pass.food_items
          .map((item) => `${item.name} x${item.quantity}`)
          .join(', ') || 'None',
        showDate: pass.show_date,
        showTime: pass.show_time,
        cinema: pass.cinema,
        screen: pass.screen,
        paymentId: pass.payment_reference,
        issuedAt: new Date(pass.issued_at).toLocaleString('en-IN'),
        demoOnly: true,
      })
      setPaymentOpen(false)
      setSession(null)
      setPage('confirmation')
      await loadSeats(movie.id)
    } catch {
      setMessage('Demo checkout could not reach the server. Please try again.')
    } finally {
      setPaymentProcessing(false)
    }
  }

  async function releaseSession() {
    if (!session) return
    await fetch(`/sessions/${session.session_id}`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: auth?.user?.id || getDemoOwnerId() }),
    })
    setSession(null)
    setPage('book')
  }

  function activateDemoMode(event) {
    event.preventDefault()
    const name = demoNameInput.trim()
    if (!name) return
    setDemoCustomerName(name)
    setDemoMode(true)
    setDemoNameOpen(false)
    setDemoNameInput('')
    setMessage('Demo mode is active. Demo passes are simulated and are not valid for real entry.')
  }

  async function exitDemoMode() {
    if (session) await releaseSession()
    setDemoMode(false)
    setDemoCustomerName('')
    setPaymentOpen(false)
  }

  async function submitAuth(event) {
    event.preventDefault()
    if (systemStatus && (!systemStatus.mongo_connected || !systemStatus.auth_enabled)) {
      setMessage('Account service unavailable: MongoDB or JWT is not connected')
      return
    }
    setAuthSubmitting(true)
    setMessage('')
    const path = authMode === 'register' ? '/auth/register' : '/auth/login'
    const payload = authMode === 'register'
      ? authForm
      : { email: authForm.email, password: authForm.password }
    const response = await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    const body = await response.json()
    if (!response.ok) {
      setMessage(body.error || 'Authentication failed')
      setAuthSubmitting(false)
      return
    }
    localStorage.setItem('cinema-auth', JSON.stringify(body))
    authRef.current = body
    setAuth(body)
    setAuthSubmitting(false)
    setAuthOpen(false)
  }

  function logout() {
    localStorage.removeItem('cinema-auth')
    authRef.current = null
    setAuth(null)
  }

  function recommendSeats() {
    if (!movie) return
    const rows = Array.from({ length: movie.rows }, (_, row) => String.fromCharCode(65 + row))
    const middleRow = rows[Math.floor(rows.length / 2)]
    const middleSeat = Math.ceil(movie.seats_per_row / 2)
    const size = Math.min(3, movie.seats_per_row)
    const recommended = Array.from({ length: size }, (_, index) => `${middleRow}${middleSeat - 1 + index}`)
    setMessage(`Best seats: ${recommended.join(', ')} — central position with a clear view`)
  }

  const statusBySeat = useMemo(() => Object.fromEntries(seats.map((seat) => [
    seat.seat_id,
    seat.confirmed ? 'booked' : 'held',
  ])), [seats])

  const foodTotal = foodMenu.reduce((total, item) => total + (cart[item.id] || 0) * item.price, 0)
  const ticketPrice = 350
  const fees = session ? 48 : 0
  const total = ticketPrice + fees + foodTotal
  return (
    <div className="app-shell">
      <header className="topbar">
        <button className="brand" onClick={() => setPage('book')} aria-label="Go to booking">
          <span className="brand-mark">C</span>
          <span>CINEMORA</span>
        </button>
        <nav className="main-nav" aria-label="Main navigation">
          <button className={page === 'discover' ? 'active' : ''} onClick={() => setPage('discover')}>Movies</button>
          <button onClick={() => setPage('book')}>Cinemas</button>
          <button onClick={() => setPage('offers')}>Offers</button>
        </nav>
        <div className="header-actions">
          {demoMode ? (
            <button className="demo-mode-button active" onClick={exitDemoMode}>Exit demo mode</button>
          ) : (
            <button className="demo-mode-button" onClick={() => setDemoNameOpen(true)}>Demo mode</button>
          )}
          {auth ? (
            <div className="user-menu">
              <button className="avatar" title={auth.user.full_name}>{auth.user.full_name?.[0] || 'U'}</button>
              <button className="logout" onClick={logout}>Log out</button>
            </div>
          ) : (
            <button className="signin" onClick={() => setAuthOpen(true)}>Sign in</button>
          )}
        </div>
      </header>
      {toast && <div className="mini-toast" role="status">{toast}</div>}
      {demoMode && (
        <div className="demo-banner" role="status">
          Demo mode · No real payment · Demo passes are not valid for entry
          <strong>Billing name: {demoCustomerName}</strong>
        </div>
      )}

      {page === 'discover' ? (
        <main className="discover-page">
          <div className="page-heading">
            <div><p className="eyebrow">Now showing</p><h1>Discover movies</h1></div>
          </div>
          <div className="movie-grid">
            {movies.map((item, index) => (
              <button key={item.id} className="poster-card" onClick={() => { setMovie(item); setPage('book') }}>
                <div className={`poster poster-${index % 2}`}>
                  <span>{item.title?.[0] || 'C'}</span>
                </div>
                <h3>{item.title}</h3>
                <p>English · {index ? 'Sci-fi Adventure' : 'Sci-fi Thriller'}</p>
              </button>
            ))}
          </div>
        </main>
      ) : page === 'confirmation' ? (
        <main className="confirmation-page">
          <div className="success-icon">✓</div>
          <p className="eyebrow">{ticket?.demoOnly ? 'Demo checkout complete' : 'Booking confirmed'}</p>
          <h1>You’re all set for {movie?.title}</h1>
          <p className="muted">
            {ticket?.demoOnly
              ? 'Simulated payment complete. No money was charged; this demo QR is not valid for real cinema entry.'
              : 'Your payment was verified. Keep your QR ticket ready for entry.'}
          </p>
          {ticket?.id && (
            <>
              <div className={`ticket-card ${ticket.demoOnly ? 'demo-ticket' : ''}`}>
                <div className="ticket-qr">
                  {ticketQrUrl ? (
                    <img src={ticketQrUrl} alt={ticket.demoOnly ? 'Demo-only QR code, not valid for entry' : 'QR code for ticket entry'} />
                  ) : (
                    <span>Generating QR</span>
                  )}
                </div>
                <div>
                  <strong>{ticket.movie}</strong>
                  <p>{ticket.customerName}</p>
                  <p>{ticket.seat} · {ticket.showDate} · {ticket.showTime} · {ticket.screen}</p>
                  <small>{ticket.demoOnly ? `DEMO ONLY · NO CHARGE · INR ${ticket.amount}` : `Paid · INR ${ticket.amount}`}</small>
                  {ticket.demoOnly && <small className="demo-warning">NOT VALID FOR ENTRY</small>}
                </div>
              </div>
              <button
                className="primary-button"
                onClick={downloadTicketPDF}
                disabled={!ticketQrDataUrl || pdfGenerating}
              >
                {pdfGenerating ? 'Preparing PDF...' : 'Download ticket PDF'}
              </button>
            </>
          )}
          <button className="primary-button" onClick={() => setPage('book')}>Book another seat</button>
        </main>
      ) : (
        <main className="booking-page">
          <aside className="booking-sidebar">
            <div className="movie-summary">
              <div className="mini-poster">{movie?.title?.[0] || 'C'}</div>
              <div>
                <h2>{movie?.title || 'Loading...'}</h2>
                <p>English · UA</p>
              </div>
            </div>
            <div className="summary-row"><span>Date</span><strong>{showDate}</strong></div>
            <div className="summary-row"><span>Showtime</span><strong>7:30 PM</strong></div>
            <div className="summary-row"><span>Theatre</span><strong>Cinemora Central</strong></div>

            <button className="recommend-button" onClick={recommendSeats}>
              <span>✦</span>
              <span><strong>Find the best seats</strong><small>Smart recommendation</small></span>
            </button>

            <div className="food-title">
              <span>Add something tasty</span>
              <button onClick={() => setCart({})}>Clear</button>
            </div>
            <div className="food-list">
              {foodMenu.map((item) => (
                <div className="food-item" key={item.id}>
                  <div className="food-emoji">{item.emoji}</div>
                  <div className="food-info"><strong>{item.name}</strong><small>{item.detail}</small></div>
                  <div className="food-control">
                    <span>₹{item.price}</span>
                    {(cart[item.id] || 0) > 0 ? (
                      <div className="stepper">
                        <button onClick={() => setCart((old) => ({ ...old, [item.id]: Math.max(0, old[item.id] - 1) }))}>−</button>
                        <b>{cart[item.id]}</b>
                        <button onClick={() => setCart((old) => ({ ...old, [item.id]: old[item.id] + 1 }))}>+</button>
                      </div>
                    ) : (
                      <button className="add-button" onClick={() => setCart((old) => ({ ...old, [item.id]: 1 }))}>Add</button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </aside>

          <section className="seat-section">
            <div className="seat-heading">
              <div>
                <p className="eyebrow">Cinemora Central · Audi 1</p>
                <h1>Select your seats</h1>
              </div>
              {session && (
                <div className="countdown">
                  <span>Held for</span>
                  <strong>{String(Math.floor(countdown / 60)).padStart(2, '0')}:{String(countdown % 60).padStart(2, '0')}</strong>
                </div>
              )}
            </div>

            <div className="seat-legend" aria-label="Seat legend">
              <span><i className="legend available" />Available</span>
              <span><i className="legend selected" />Selected</span>
              <span><i className="legend held" />Reserved</span>
              <span><i className="legend booked" />Booked</span>
              <span><i className="legend accessible" />Accessible</span>
            </div>

            <div className="screen" aria-hidden="true"><span>SCREEN</span></div>
            {movie ? (
              <div className="seat-map" role="grid" aria-label="Seat selection">
                {Array.from({ length: movie.rows }, (_, row) => {
                  const rowLetter = String.fromCharCode(65 + row)
                  return (
                    <div className="seat-row" role="row" key={row}>
                      <span className="row-label">{rowLetter}</span>
                      <div className="seat-row-items">
                        {Array.from({ length: movie.seats_per_row }, (_, number) => {
                          const seatId = `${rowLetter}${number + 1}`
                          const status = statusBySeat[seatId] || 'available'
                          const selected = session?.seat_id === seatId
                          const accessible = row === 0 && number === 0
                          return (
                            <button
                              key={seatId}
                              role="gridcell"
                              aria-label={`Seat ${seatId}, ${selected ? 'selected' : status}`}
                              disabled={status !== 'available' && !selected}
                              onClick={() => holdSeat(seatId)}
                              className={`seat ${selected ? 'selected' : status} ${accessible ? 'accessible' : ''}`}
                            >
                              {accessible ? '♿' : seatId.slice(1)}
                            </button>
                          )
                        })}
                      </div>
                      <span className="row-label">{rowLetter}</span>
                    </div>
                  )
                })}
              </div>
            ) : <p className="loading">Loading seating layout...</p>}

            {message && <p className="status-message" role="status">{message}</p>}

            <div className="checkout-bar">
              <div className="selection-summary">
                <span>{session ? '1 ticket selected' : 'Select a seat'}</span>
                {session && <strong>{session.seat_id}</strong>}
              </div>
              <div className="price-summary">
                <span>Total</span>
                <strong>₹{total}</strong>
              </div>
              {session ? (
                <div className="checkout-actions">
                  <button className="cancel-button" onClick={releaseSession}>Cancel</button>
                  <button className="primary-button" onClick={confirmSession}>Continue to pay</button>
                </div>
              ) : (
                <button className="primary-button" disabled>Select seats</button>
              )}
            </div>
          </section>
        </main>
      )}

      {paymentOpen && (
        <div className="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="payment-title">
          <div className="payment-modal">
            <button className="modal-close" onClick={() => setPaymentOpen(false)} aria-label="Close">×</button>
            <p className="eyebrow">{demoMode ? 'Demo checkout' : 'Secure checkout'}</p>
            <h2 id="payment-title">Complete your booking</h2>
            <div className="payment-summary">
              <div><span>Movie</span><strong>{movie?.title}</strong></div>
              <div><span>Seat</span><strong>{session?.seat_id}</strong></div>
              <div><span>Food</span><strong>₹{foodTotal}</strong></div>
              <div><span>Convenience fee</span><strong>₹{fees}</strong></div>
            </div>
            <div className="payment-total"><span>Total payable</span><strong>₹{total}</strong></div>
            <button className="primary-button wide" onClick={demoMode ? payWithDemo : payWithRazorpay} disabled={paymentProcessing}>
              {paymentProcessing
                ? (demoMode ? 'Completing demo checkout...' : 'Opening secure checkout...')
                : (demoMode ? `Complete demo payment · ₹${total}` : `Pay ₹${total}`)}
            </button>
            <p className="payment-note">
              {demoMode
                ? 'This simulates a successful payment. No money is charged. The generated demo pass and QR are not valid for real entry.'
                : 'Payments are processed securely by Razorpay. Your seat is confirmed only after signature verification.'}
            </p>
          </div>
        </div>
      )}

      {demoNameOpen && (
        <div className="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="demo-title">
          <div className="auth-modal">
            <button className="modal-close" onClick={() => setDemoNameOpen(false)} aria-label="Close">×</button>
            <p className="eyebrow">Try the booking flow</p>
            <h2 id="demo-title">Start demo mode</h2>
            <p className="payment-note demo-modal-note">No real payment will be taken. Demo passes cannot be used for cinema entry.</p>
            <form onSubmit={activateDemoMode}>
              <label>Billing name<input required autoFocus maxLength={80} value={demoNameInput} onChange={(event) => setDemoNameInput(event.target.value)} /></label>
              <button className="primary-button wide" type="submit">Continue in demo mode</button>
            </form>
          </div>
        </div>
      )}

      {authOpen && (
        <div className="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="auth-title">
          <div className="auth-modal">
            <button className="modal-close" onClick={() => setAuthOpen(false)} aria-label="Close">×</button>
            <p className="eyebrow">Cinemora</p>
            <h2 id="auth-title">{authMode === 'login' ? 'Welcome back' : 'Create account'}</h2>
            <div className="auth-tabs">
              <button className={authMode === 'login' ? 'active' : ''} onClick={() => setAuthMode('login')}>Sign in</button>
              <button className={authMode === 'register' ? 'active' : ''} onClick={() => setAuthMode('register')}>Register</button>
            </div>
            {systemStatus && (!systemStatus.mongo_connected || !systemStatus.auth_enabled) && (
              <p className="status-message">Account service is unavailable until MongoDB Atlas and JWT configuration are connected.</p>
            )}
            <form onSubmit={submitAuth}>
              {authMode === 'register' && (
                <>
                  <label>Full name<input required value={authForm.full_name} onChange={(event) => setAuthForm({ ...authForm, full_name: event.target.value })} /></label>
                  <label>Phone<input value={authForm.phone} onChange={(event) => setAuthForm({ ...authForm, phone: event.target.value })} /></label>
                </>
              )}
              <label>Email<input type="email" required value={authForm.email} onChange={(event) => setAuthForm({ ...authForm, email: event.target.value })} /></label>
              <label>Password<input type="password" required minLength={6} value={authForm.password} onChange={(event) => setAuthForm({ ...authForm, password: event.target.value })} /></label>
              <button className="primary-button wide" type="submit" disabled={authSubmitting}>
                {authSubmitting ? 'Please wait...' : (authMode === 'login' ? 'Sign in' : 'Create account')}
              </button>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

export default App
