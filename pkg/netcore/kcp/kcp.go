// Package kcp - A Fast and Reliable ARQ Protocol
package kcp

import (
	"encoding/binary"
)

const (
	IKCP_RTO_NDL     = 30  // no delay min rto
	IKCP_RTO_MIN     = 100 // normal min rto
	IKCP_RTO_DEF     = 200
	IKCP_RTO_MAX     = 60000
	IKCP_CMD_PUSH    = 81 // cmd: push data
	IKCP_CMD_ACK     = 82 // cmd: ack
	IKCP_CMD_WASK    = 83 // cmd: window probe (ask)
	IKCP_CMD_WINS    = 84 // cmd: window size (tell)
	IKCP_ASK_SEND    = 1  // need to send IKCP_CMD_WASK
	IKCP_ASK_TELL    = 2  // need to send IKCP_CMD_WINS
	IKCP_WND_SND     = 32
	IKCP_WND_RCV     = 32
	IKCP_MTU_DEF     = 1400
	IKCP_ACK_FAST    = 3
	IKCP_INTERVAL    = 100
	IKCP_OVERHEAD    = 24
	IKCP_DEADLINK    = 20
	IKCP_THRESH_INIT = 2
	IKCP_THRESH_MIN  = 2
	IKCP_PROBE_INIT  = 7000   // 7 secs to probe window size
	IKCP_PROBE_LIMIT = 120000 // up to 120 secs to probe window
)

// output_callback is a prototype which ought capture conn and call conn.Write
type output_callback func(buf []byte, size int)

/* encode 8 bits unsigned int */
func ikcp_encode8u(p []byte, c byte) []byte {
	p[0] = c
	return p[1:]
}

/* decode 8 bits unsigned int */
func ikcp_decode8u(p []byte, c *byte) []byte {
	*c = p[0]
	return p[1:]
}

/* encode 16 bits unsigned int (lsb) */
func ikcp_encode16u(p []byte, w uint16) []byte {
	binary.LittleEndian.PutUint16(p, w)
	return p[2:]
}

/* decode 16 bits unsigned int (lsb) */
func ikcp_decode16u(p []byte, w *uint16) []byte {
	*w = binary.LittleEndian.Uint16(p)
	return p[2:]
}

/* encode 32 bits unsigned int (lsb) */
func ikcp_encode32u(p []byte, l uint32) []byte {
	binary.LittleEndian.PutUint32(p, l)
	return p[4:]
}

/* decode 32 bits unsigned int (lsb) */
func ikcp_decode32u(p []byte, l *uint32) []byte {
	*l = binary.LittleEndian.Uint32(p)
	return p[4:]
}

func _imin_(a, b uint32) uint32 {
	if a <= b {
		return a
	}
	return b
}

func _imax_(a, b uint32) uint32 {
	if a >= b {
		return a
	}
	return b
}

func _ibound_(lower, middle, upper uint32) uint32 {
	return _imin_(_imax_(lower, middle), upper)
}

func _itimediff(later, earlier uint32) int32 {
	return (int32)(later - earlier)
}

// segment defines a KCP segment
type segment struct {
	conv     uint32
	cmd      uint8
	frg      uint8
	wnd      uint16
	ts       uint32
	sn       uint32
	una      uint32
	rto      uint32
	xmit     uint32
	resendts uint32
	fastack  uint32
	data     []byte
}

// encode a segment into buffer
func (seg *segment) encode(ptr []byte) []byte {
	ptr = ikcp_encode32u(ptr, seg.conv)
	ptr = ikcp_encode8u(ptr, seg.cmd)
	ptr = ikcp_encode8u(ptr, seg.frg)
	ptr = ikcp_encode16u(ptr, seg.wnd)
	ptr = ikcp_encode32u(ptr, seg.ts)
	ptr = ikcp_encode32u(ptr, seg.sn)
	ptr = ikcp_encode32u(ptr, seg.una)
	ptr = ikcp_encode32u(ptr, uint32(len(seg.data)))

	return ptr
}

// KCP defines a single KCP connection
type KCP struct {
	conv, mtu, mss, state                  uint32
	snd_una, snd_nxt, rcv_nxt              uint32
	ssthresh                               uint32
	rx_rttvar, rx_srtt                     int32
	rx_rto, rx_minrto                      uint32
	snd_wnd, rcv_wnd, rmt_wnd, cwnd, probe uint32
	interval, ts_flush                     uint32
	nodelay, updated                       uint32
	ts_probe, probe_wait                   uint32
	dead_link, incr                        uint32

	fastresend     int32
	nocwnd, stream int32

	snd_queue []segment
	rcv_queue []segment
	snd_buf   []segment
	rcv_buf   []segment

	acklist []ackItem

	buffer []byte
	output output_callback

	current uint32
}

type ackItem struct {
	sn uint32
	ts uint32
}

// NewKCP create a new kcp control object, 'conv' must equal in two endpoint
// from the same connection.
func NewKCP(conv uint32, output output_callback) *KCP {
	kcp := new(KCP)
	kcp.conv = conv
	kcp.snd_wnd = IKCP_WND_SND
	kcp.rcv_wnd = IKCP_WND_RCV
	kcp.rmt_wnd = IKCP_WND_RCV
	kcp.mtu = IKCP_MTU_DEF
	kcp.mss = kcp.mtu - IKCP_OVERHEAD
	kcp.buffer = make([]byte, (kcp.mtu+IKCP_OVERHEAD)*3)
	kcp.rx_rto = IKCP_RTO_DEF
	kcp.rx_minrto = IKCP_RTO_MIN
	kcp.interval = IKCP_INTERVAL
	kcp.ts_flush = IKCP_INTERVAL
	kcp.ssthresh = IKCP_THRESH_INIT
	kcp.dead_link = IKCP_DEADLINK
	kcp.output = output
	return kcp
}

// newSegment creates a KCP segment
func (kcp *KCP) newSegment(size int) (seg segment) {
	if size > 0 {
		seg.data = make([]byte, size)
	}
	return
}

// PeekSize checks the size of next message in the recv queue
func (kcp *KCP) PeekSize() (length int) {
	if len(kcp.rcv_queue) == 0 {
		return -1
	}

	seg := &kcp.rcv_queue[0]
	if seg.frg == 0 {
		return len(seg.data)
	}

	if len(kcp.rcv_queue) < int(seg.frg+1) {
		return -1
	}

	for k := range kcp.rcv_queue {
		seg := &kcp.rcv_queue[k]
		length += len(seg.data)
		if seg.frg == 0 {
			break
		}
	}
	return
}

// Recv is user/upper level recv: returns size, returns below zero for EAGAIN
func (kcp *KCP) Recv(buffer []byte) (n int) {
	if len(kcp.rcv_queue) == 0 {
		return -1
	}

	peeksize := kcp.PeekSize()
	if peeksize < 0 {
		return -2
	}

	if peeksize > len(buffer) {
		return -3
	}

	var fast_recover bool
	if len(kcp.rcv_queue) >= int(kcp.rcv_wnd) {
		fast_recover = true
	}

	// merge fragment
	count := 0
	for k := range kcp.rcv_queue {
		seg := &kcp.rcv_queue[k]
		copy(buffer, seg.data)
		buffer = buffer[len(seg.data):]
		n += len(seg.data)
		count++
		if seg.frg == 0 {
			break
		}
	}
	if count > 0 {
		kcp.rcv_queue = kcp.remove_front(kcp.rcv_queue, count)
	}

	// move available data from rcv_buf -> rcv_queue
	count = 0
	for k := range kcp.rcv_buf {
		seg := &kcp.rcv_buf[k]
		if seg.sn == kcp.rcv_nxt && len(kcp.rcv_queue) < int(kcp.rcv_wnd) {
			kcp.rcv_nxt++
			count++
		} else {
			break
		}
	}

	if count > 0 {
		kcp.rcv_queue = append(kcp.rcv_queue, kcp.rcv_buf[:count]...)
		kcp.rcv_buf = kcp.remove_front(kcp.rcv_buf, count)
	}

	// fast recover
	if len(kcp.rcv_queue) < int(kcp.rcv_wnd) && fast_recover {
		// ready to send back IKCP_CMD_WINS in ikcp_flush
		// tell remote my window size
		kcp.probe |= IKCP_ASK_TELL
	}
	return
}

// Send is user/upper level send, returns below zero for error
func (kcp *KCP) Send(buffer []byte) int {
	var count int
	if len(buffer) == 0 {
		return -1
	}

	// append to previous segment in streaming mode (if possible)
	if kcp.stream != 0 {
		n := len(kcp.snd_queue)
		if n > 0 {
			seg := &kcp.snd_queue[n-1]
			if len(seg.data) < int(kcp.mss) {
				capacity := int(kcp.mss) - len(seg.data)
				extend := capacity
				if len(buffer) < capacity {
					extend = len(buffer)
				}

				// grow slice, the underlying cap is guaranteed to
				// be larger than kcp.mss
				oldlen := len(seg.data)
				seg.data = seg.data[:oldlen+extend]
				copy(seg.data[oldlen:], buffer)
				buffer = buffer[extend:]
			}
		}

		if len(buffer) == 0 {
			return 0
		}
	}

	if len(buffer) <= int(kcp.mss) {
		count = 1
	} else {
		count = (len(buffer) + int(kcp.mss) - 1) / int(kcp.mss)
	}

	if count > 255 {
		return -2
	}

	if count == 0 {
		count = 1
	}

	for i := 0; i < count; i++ {
		var size int
		if len(buffer) > int(kcp.mss) {
			size = int(kcp.mss)
		} else {
			size = len(buffer)
		}
		seg := kcp.newSegment(size)
		copy(seg.data, buffer[:size])
		if kcp.stream == 0 { // message mode
			seg.frg = uint8(count - i - 1)
		} else { // stream mode
			seg.frg = 0
		}
		kcp.snd_queue = append(kcp.snd_queue, seg)
		buffer = buffer[size:]
	}
	return 0
}

func (kcp *KCP) update_ack(rtt int32) {
	// https://tools.ietf.org/html/rfc6298
	var rto uint32
	if kcp.rx_srtt == 0 {
		kcp.rx_srtt = rtt
		kcp.rx_rttvar = rtt >> 1
	} else {
		delta := rtt - kcp.rx_srtt
		kcp.rx_srtt += delta >> 3
		if delta < 0 {
			delta = -delta
		}
		if rtt < kcp.rx_srtt-kcp.rx_rttvar {
			// if the new RTT sample is below the bottom of the range of
			// what an RTT measurement is expected to be.
			// give an 8x reduced weight versus its normal weighting
			kcp.rx_rttvar += (delta - kcp.rx_rttvar) >> 5
		} else {
			kcp.rx_rttvar += (delta - kcp.rx_rttvar) >> 2
		}
	}
	rto = uint32(kcp.rx_srtt) + _imax_(kcp.interval, uint32(kcp.rx_rttvar)<<2)
	kcp.rx_rto = _ibound_(kcp.rx_minrto, rto, IKCP_RTO_MAX)
}

func (kcp *KCP) shrink_buf() {
	if len(kcp.snd_buf) > 0 {
		seg := &kcp.snd_buf[0]
		kcp.snd_una = seg.sn
	} else {
		kcp.snd_una = kcp.snd_nxt
	}
}

func (kcp *KCP) parse_ack(sn uint32) {
	if _itimediff(sn, kcp.snd_una) < 0 || _itimediff(sn, kcp.snd_nxt) >= 0 {
		return
	}

	for k := range kcp.snd_buf {
		seg := &kcp.snd_buf[k]
		if sn == seg.sn {
			copy(kcp.snd_buf[k:], kcp.snd_buf[k+1:])
			kcp.snd_buf[len(kcp.snd_buf)-1] = segment{}
			kcp.snd_buf = kcp.snd_buf[:len(kcp.snd_buf)-1]
			break
		}
		if _itimediff(sn, seg.sn) < 0 {
			break
		}
	}
}

func (kcp *KCP) parse_fastack(sn uint32) {
	if _itimediff(sn, kcp.snd_una) < 0 || _itimediff(sn, kcp.snd_nxt) >= 0 {
		return
	}

	for k := range kcp.snd_buf {
		seg := &kcp.snd_buf[k]
		if _itimediff(sn, seg.sn) < 0 {
			break
		} else if sn != seg.sn {
			seg.fastack++
		}
	}
}

func (kcp *KCP) parse_una(una uint32) {
	count := 0
	for k := range kcp.snd_buf {
		seg := &kcp.snd_buf[k]
		if _itimediff(una, seg.sn) > 0 {
			count++
		} else {
			break
		}
	}
	if count > 0 {
		kcp.snd_buf = kcp.remove_front(kcp.snd_buf, count)
	}
}

// ack append
func (kcp *KCP) ack_push(sn, ts uint32) {
	kcp.acklist = append(kcp.acklist, ackItem{sn, ts})
}

func (kcp *KCP) parse_data(newseg segment) {
	sn := newseg.sn
	if _itimediff(sn, kcp.rcv_nxt+kcp.rcv_wnd) >= 0 ||
		_itimediff(sn, kcp.rcv_nxt) < 0 {
		return
	}

	n := len(kcp.rcv_buf) - 1
	insert_idx := 0
	repeat := false
	for i := n; i >= 0; i-- {
		seg := &kcp.rcv_buf[i]
		if seg.sn == sn {
			repeat = true
			break
		}
		if _itimediff(sn, seg.sn) > 0 {
			insert_idx = i + 1
			break
		}
	}

	if !repeat {
		if insert_idx == n+1 {
			kcp.rcv_buf = append(kcp.rcv_buf, newseg)
		} else {
			kcp.rcv_buf = append(kcp.rcv_buf, segment{})
			copy(kcp.rcv_buf[insert_idx+1:], kcp.rcv_buf[insert_idx:])
			kcp.rcv_buf[insert_idx] = newseg
		}
	} else {

	}

	// move available data from rcv_buf -> rcv_queue
	count := 0
	for k := range kcp.rcv_buf {
		seg := &kcp.rcv_buf[k]
		if seg.sn == kcp.rcv_nxt && len(kcp.rcv_queue) < int(kcp.rcv_wnd) {
			kcp.rcv_nxt++
			count++
		} else {
			break
		}
	}
	if count > 0 {
		kcp.rcv_queue = append(kcp.rcv_queue, kcp.rcv_buf[:count]...)
		kcp.rcv_buf = kcp.remove_front(kcp.rcv_buf, count)
	}
}

// Input when you received a low level packet (eg. UDP packet), call it
// regular indicates a regular packet has received(not from FEC)
func (kcp *KCP) Input(data []byte, regular, ackNoDelay bool) int {
	snd_una := kcp.snd_una
	if len(data) < IKCP_OVERHEAD {
		return -1
	}

	var maxack uint32
	var lastackts uint32
	var flag int
	var inSegs uint64

	for {
		var ts, sn, length, una, conv uint32
		var wnd uint16
		var cmd, frg uint8

		if len(data) < int(IKCP_OVERHEAD) {
			break
		}

		data = ikcp_decode32u(data, &conv)
		if conv != kcp.conv {
			return -1
		}

		data = ikcp_decode8u(data, &cmd)
		data = ikcp_decode8u(data, &frg)
		data = ikcp_decode16u(data, &wnd)
		data = ikcp_decode32u(data, &ts)
		data = ikcp_decode32u(data, &sn)
		data = ikcp_decode32u(data, &una)
		data = ikcp_decode32u(data, &length)
		if len(data) < int(length) {
			return -2
		}

		if cmd != IKCP_CMD_PUSH && cmd != IKCP_CMD_ACK &&
			cmd != IKCP_CMD_WASK && cmd != IKCP_CMD_WINS {
			return -3
		}

		// only trust window updates from regular packets. i.e: latest update
		if regular {
			kcp.rmt_wnd = uint32(wnd)
		}
		kcp.parse_una(una)
		kcp.shrink_buf()

		if cmd == IKCP_CMD_ACK {
			kcp.parse_ack(sn)
			kcp.shrink_buf()
			if flag == 0 {
				flag = 1
				maxack = sn
				lastackts = ts
			} else if _itimediff(sn, maxack) > 0 {
				maxack = sn
				lastackts = ts
			}
		} else if cmd == IKCP_CMD_PUSH {
			if _itimediff(sn, kcp.rcv_nxt+kcp.rcv_wnd) < 0 {
				kcp.ack_push(sn, ts)
				if _itimediff(sn, kcp.rcv_nxt) >= 0 {
					seg := kcp.newSegment(int(length))
					seg.conv = conv
					seg.cmd = cmd
					seg.frg = frg
					seg.wnd = wnd
					seg.ts = ts
					seg.sn = sn
					seg.una = una
					copy(seg.data, data[:length])
					kcp.parse_data(seg)
				}
			}
		} else if cmd == IKCP_CMD_WASK {
			// ready to send back IKCP_CMD_WINS in Ikcp_flush
			// tell remote my window size
			kcp.probe |= IKCP_ASK_TELL
		} else if cmd == IKCP_CMD_WINS {
			// do nothing
		} else {
			return -3
		}

		inSegs++
		data = data[length:]
	}

	if flag != 0 && regular {
		kcp.parse_fastack(maxack)
		current := kcp.current
		if _itimediff(current, lastackts) >= 0 {
			kcp.update_ack(_itimediff(current, lastackts))
		}
	}

	if _itimediff(kcp.snd_una, snd_una) > 0 {
		if kcp.cwnd < kcp.rmt_wnd {
			mss := kcp.mss
			if kcp.cwnd < kcp.ssthresh {
				kcp.cwnd++
				kcp.incr += mss
			} else {
				if kcp.incr < mss {
					kcp.incr = mss
				}
				kcp.incr += (mss*mss)/kcp.incr + (mss / 16)
				if (kcp.cwnd+1)*mss <= kcp.incr {
					kcp.cwnd++
				}
			}
			if kcp.cwnd > kcp.rmt_wnd {
				kcp.cwnd = kcp.rmt_wnd
				kcp.incr = kcp.rmt_wnd * mss
			}
		}
	}

	if ackNoDelay && len(kcp.acklist) > 0 { // ack immediately
		kcp.flush(true)
	}
	return 0
}

func (kcp *KCP) wnd_unused() uint16 {
	if len(kcp.rcv_queue) < int(kcp.rcv_wnd) {
		return uint16(int(kcp.rcv_wnd) - len(kcp.rcv_queue))
	}
	return 0
}

// flush pending data
func (kcp *KCP) flush(ackOnly bool) {
	var seg segment
	seg.conv = kcp.conv
	seg.cmd = IKCP_CMD_ACK
	seg.wnd = kcp.wnd_unused()
	seg.una = kcp.rcv_nxt

	buffer := kcp.buffer
	// flush acknowledges
	ptr := buffer
	for i, ack := range kcp.acklist {
		size := len(buffer) - len(ptr)
		if size+IKCP_OVERHEAD > int(kcp.mtu) {
			kcp.output(buffer, size)
			ptr = buffer
		}
		// filter jitters caused by bufferbloat
		if ack.sn >= kcp.rcv_nxt || len(kcp.acklist)-1 == i {
			seg.sn, seg.ts = ack.sn, ack.ts
			ptr = seg.encode(ptr)
		}
	}
	kcp.acklist = kcp.acklist[0:0]

	if ackOnly { // flash remain ack segments
		size := len(buffer) - len(ptr)
		if size > 0 {
			kcp.output(buffer, size)
		}
		return
	}

	// probe window size (if remote window size equals zero)
	if kcp.rmt_wnd == 0 {
		current := kcp.current
		if kcp.probe_wait == 0 {
			kcp.probe_wait = IKCP_PROBE_INIT
			kcp.ts_probe = current + kcp.probe_wait
		} else {
			if _itimediff(current, kcp.ts_probe) >= 0 {
				if kcp.probe_wait < IKCP_PROBE_INIT {
					kcp.probe_wait = IKCP_PROBE_INIT
				}
				kcp.probe_wait += kcp.probe_wait / 2
				if kcp.probe_wait > IKCP_PROBE_LIMIT {
					kcp.probe_wait = IKCP_PROBE_LIMIT
				}
				kcp.ts_probe = current + kcp.probe_wait
				kcp.probe |= IKCP_ASK_SEND
			}
		}
	} else {
		kcp.ts_probe = 0
		kcp.probe_wait = 0
	}

	// flush window probing commands
	if (kcp.probe & IKCP_ASK_SEND) != 0 {
		seg.cmd = IKCP_CMD_WASK
		size := len(buffer) - len(ptr)
		if size+IKCP_OVERHEAD > int(kcp.mtu) {
			kcp.output(buffer, size)
			ptr = buffer
		}
		ptr = seg.encode(ptr)
	}

	// flush window probing commands
	if (kcp.probe & IKCP_ASK_TELL) != 0 {
		seg.cmd = IKCP_CMD_WINS
		size := len(buffer) - len(ptr)
		if size+IKCP_OVERHEAD > int(kcp.mtu) {
			kcp.output(buffer, size)
			ptr = buffer
		}
		ptr = seg.encode(ptr)
	}

	kcp.probe = 0

	// calculate window size
	cwnd := _imin_(kcp.snd_wnd, kcp.rmt_wnd)
	if kcp.nocwnd == 0 {
		cwnd = _imin_(kcp.cwnd, cwnd)
	}

	// sliding window, controlled by snd_nxt && sna_una+cwnd
	newSegsCount := 0
	for k := range kcp.snd_queue {
		if _itimediff(kcp.snd_nxt, kcp.snd_una+cwnd) >= 0 {
			break
		}
		newseg := kcp.snd_queue[k]
		newseg.conv = kcp.conv
		newseg.cmd = IKCP_CMD_PUSH
		newseg.sn = kcp.snd_nxt
		kcp.snd_buf = append(kcp.snd_buf, newseg)
		kcp.snd_nxt++
		newSegsCount++
		kcp.snd_queue[k].data = nil
	}
	if newSegsCount > 0 {
		kcp.snd_queue = kcp.remove_front(kcp.snd_queue, newSegsCount)
	}

	// calculate resent
	resent := uint32(kcp.fastresend)
	if kcp.fastresend <= 0 {
		resent = 0xffffffff
	}

	// check for retransmissions
	current := kcp.current
	var change, lost uint64
	for k := range kcp.snd_buf {
		segment := &kcp.snd_buf[k]
		needsend := false
		if segment.xmit == 0 { // initial transmit
			needsend = true
			segment.rto = kcp.rx_rto
			segment.resendts = current + segment.rto
		} else if _itimediff(current, segment.resendts) >= 0 { // RTO
			needsend = true
			if kcp.nodelay == 0 {
				segment.rto += kcp.rx_rto
			} else {
				segment.rto += kcp.rx_rto / 2
			}
			segment.resendts = current + segment.rto
			lost++
		} else if segment.fastack >= resent { // fast retransmit
			needsend = true
			segment.fastack = 0
			segment.rto = kcp.rx_rto
			segment.resendts = current + segment.rto
			change++
		} else if segment.fastack > 0 && newSegsCount == 0 { // early retransmit
			needsend = true
			segment.fastack = 0
			segment.rto = kcp.rx_rto
			segment.resendts = current + segment.rto
			change++
		}

		if needsend {
			segment.xmit++
			segment.ts = current
			segment.wnd = seg.wnd
			segment.una = seg.una

			size := len(buffer) - len(ptr)
			need := IKCP_OVERHEAD + len(segment.data)

			if size+need > int(kcp.mtu) {
				kcp.output(buffer, size)
				ptr = buffer
			}

			ptr = segment.encode(ptr)
			copy(ptr, segment.data)
			ptr = ptr[len(segment.data):]

			if segment.xmit >= kcp.dead_link {
				kcp.state = 0xFFFFFFFF
			}
		}
	}

	// flash remain segments
	size := len(buffer) - len(ptr)
	if size > 0 {
		kcp.output(buffer, size)
	}

	// update ssthresh
	// rate halving, https://tools.ietf.org/html/rfc6937
	if change > 0 {
		inflight := kcp.snd_nxt - kcp.snd_una
		kcp.ssthresh = inflight / 2
		if kcp.ssthresh < IKCP_THRESH_MIN {
			kcp.ssthresh = IKCP_THRESH_MIN
		}
		kcp.cwnd = kcp.ssthresh + resent
		kcp.incr = kcp.cwnd * kcp.mss
	}

	// congestion control, https://tools.ietf.org/html/rfc5681
	if lost > 0 {
		kcp.ssthresh = cwnd / 2
		if kcp.ssthresh < IKCP_THRESH_MIN {
			kcp.ssthresh = IKCP_THRESH_MIN
		}
		kcp.cwnd = 1
		kcp.incr = kcp.mss
	}

	if kcp.cwnd < 1 {
		kcp.cwnd = 1
		kcp.incr = kcp.mss
	}
}

// Update updates state (call it repeatedly, every 10ms-100ms), or you can ask
// ikcp_check when to call it again (without ikcp_input/_send calling).
// 'current' - current timestamp in millisec.
func (kcp *KCP) Update(current uint32) {
	var slap int32

	kcp.current = current
	if kcp.updated == 0 {
		kcp.updated = 1
		kcp.ts_flush = current
	}

	slap = _itimediff(current, kcp.ts_flush)

	if slap >= 10000 || slap < -10000 {
		kcp.ts_flush = current
		slap = 0
	}

	if slap >= 0 {
		kcp.ts_flush += kcp.interval
		if _itimediff(current, kcp.ts_flush) >= 0 {
			kcp.ts_flush = current + kcp.interval
		}
		kcp.flush(false)
	}
}

// Check determines when should you invoke ikcp_update:
// returns when you should invoke ikcp_update in millisec, if there
// is no ikcp_input/_send calling. you can call ikcp_update in that
// time, instead of call update repeatly.
// Important to reduce unnacessary ikcp_update invoking. use it to
// schedule ikcp_update (eg. implementing an epoll-like mechanism,
// or optimize ikcp_update when handling massive kcp connections)
func (kcp *KCP) Check() uint32 {
	current := kcp.current
	ts_flush := kcp.ts_flush
	tm_flush := int32(0x7fffffff)
	tm_packet := int32(0x7fffffff)
	minimal := uint32(0)
	if kcp.updated == 0 {
		return current
	}

	if _itimediff(current, ts_flush) >= 10000 ||
		_itimediff(current, ts_flush) < -10000 {
		ts_flush = current
	}

	if _itimediff(current, ts_flush) >= 0 {
		return current
	}

	tm_flush = _itimediff(ts_flush, current)

	for k := range kcp.snd_buf {
		seg := &kcp.snd_buf[k]
		diff := _itimediff(seg.resendts, current)
		if diff <= 0 {
			return current
		}
		if diff < tm_packet {
			tm_packet = diff
		}
	}

	minimal = uint32(tm_packet)
	if tm_packet >= tm_flush {
		minimal = uint32(tm_flush)
	}
	if minimal >= kcp.interval {
		minimal = kcp.interval
	}

	return current + minimal
}

// SetMtu changes MTU size, default is 1400
func (kcp *KCP) SetMtu(mtu int) int {
	if mtu < 50 || mtu < IKCP_OVERHEAD {
		return -1
	}
	buffer := make([]byte, (mtu+IKCP_OVERHEAD)*3)
	if buffer == nil {
		return -2
	}
	kcp.mtu = uint32(mtu)
	kcp.mss = kcp.mtu - IKCP_OVERHEAD
	kcp.buffer = buffer
	return 0
}

// NoDelay options
// fastest: ikcp_nodelay(kcp, 1, 20, 2, 1)
// nodelay: 0:disable(default), 1:enable
// interval: internal update timer interval in millisec, default is 100ms
// resend: 0:disable fast resend(default), 1:enable fast resend
// nc: 0:normal congestion control(default), 1:disable congestion control
func (kcp *KCP) NoDelay(nodelay, interval, resend, nc int) int {
	if nodelay >= 0 {
		kcp.nodelay = uint32(nodelay)
		if nodelay != 0 {
			kcp.rx_minrto = IKCP_RTO_NDL
		} else {
			kcp.rx_minrto = IKCP_RTO_MIN
		}
	}
	if interval >= 0 {
		if interval > 5000 {
			interval = 5000
		} else if interval < 10 {
			interval = 10
		}
		kcp.interval = uint32(interval)
	}
	if resend >= 0 {
		kcp.fastresend = int32(resend)
	}
	if nc >= 0 {
		kcp.nocwnd = int32(nc)
	}
	return 0
}

// WndSize sets maximum window size: sndwnd=32, rcvwnd=32 by default
func (kcp *KCP) WndSize(sndwnd, rcvwnd int) int {
	if sndwnd > 0 {
		kcp.snd_wnd = uint32(sndwnd)
	}
	if rcvwnd > 0 {
		kcp.rcv_wnd = uint32(rcvwnd)
	}
	return 0
}

// WaitSnd gets how many packet is waiting to be sent
func (kcp *KCP) WaitSnd() int {
	return len(kcp.snd_buf) + len(kcp.snd_queue)
}

// remove front n elements from queue
func (kcp *KCP) remove_front(q []segment, n int) []segment {
	newn := copy(q, q[n:])
	for i := newn; i < len(q); i++ {
		q[i] = segment{} // manual set nil for GC
	}
	return q[:newn]
}
