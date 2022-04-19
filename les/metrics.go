package les

import (
	"github.com/ChainAAS/gendchain/metrics"
	"github.com/ChainAAS/gendchain/p2p"
)

var (
	/*	propTxnInPacketsMeter     = metrics.NewRegisteredMeter("eth/prop/txns/in/packets")
		propTxnInTrafficMeter     = metrics.NewRegisteredMeter("eth/prop/txns/in/traffic")
		propTxnOutPacketsMeter    = metrics.NewRegisteredMeter("eth/prop/txns/out/packets")
		propTxnOutTrafficMeter    = metrics.NewRegisteredMeter("eth/prop/txns/out/traffic")
		propHashInPacketsMeter    = metrics.NewRegisteredMeter("eth/prop/hashes/in/packets")
		propHashInTrafficMeter    = metrics.NewRegisteredMeter("eth/prop/hashes/in/traffic")
		propHashOutPacketsMeter   = metrics.NewRegisteredMeter("eth/prop/hashes/out/packets")
		propHashOutTrafficMeter   = metrics.NewRegisteredMeter("eth/prop/hashes/out/traffic")
		propBlockInPacketsMeter   = metrics.NewRegisteredMeter("eth/prop/blocks/in/packets")
		propBlockInTrafficMeter   = metrics.NewRegisteredMeter("eth/prop/blocks/in/traffic")
		propBlockOutPacketsMeter  = metrics.NewRegisteredMeter("eth/prop/blocks/out/packets")
		propBlockOutTrafficMeter  = metrics.NewRegisteredMeter("eth/prop/blocks/out/traffic")
		reqHashInPacketsMeter     = metrics.NewRegisteredMeter("eth/req/hashes/in/packets")
		reqHashInTrafficMeter     = metrics.NewRegisteredMeter("eth/req/hashes/in/traffic")
		reqHashOutPacketsMeter    = metrics.NewRegisteredMeter("eth/req/hashes/out/packets")
		reqHashOutTrafficMeter    = metrics.NewRegisteredMeter("eth/req/hashes/out/traffic")
		reqBlockInPacketsMeter    = metrics.NewRegisteredMeter("eth/req/blocks/in/packets")
		reqBlockInTrafficMeter    = metrics.NewMeter("eth/req/blocks/in/traffic")
		reqBlockOutPacketsMeter   = metrics.NewMeter("eth/req/blocks/out/packets")
		reqBlockOutTrafficMeter   = metrics.NewMeter("eth/req/blocks/out/traffic")
		reqHeaderInPacketsMeter   = metrics.NewMeter("eth/req/headers/in/packets")
		reqHeaderInTrafficMeter   = metrics.NewMeter("eth/req/headers/in/traffic")
		reqHeaderOutPacketsMeter  = metrics.NewMeter("eth/req/headers/out/packets")
		reqHeaderOutTrafficMeter  = metrics.NewMeter("eth/req/headers/out/traffic")
		reqBodyInPacketsMeter     = metrics.NewMeter("eth/req/bodies/in/packets")
		reqBodyInTrafficMeter     = metrics.NewMeter("eth/req/bodies/in/traffic")
		reqBodyOutPacketsMeter    = metrics.NewMeter("eth/req/bodies/out/packets")
		reqBodyOutTrafficMeter    = metrics.NewMeter("eth/req/bodies/out/traffic")
		reqStateInPacketsMeter    = metrics.NewMeter("eth/req/states/in/packets")
		reqStateInTrafficMeter    = metrics.NewMeter("eth/req/states/in/traffic")
		reqStateOutPacketsMeter   = metrics.NewMeter("eth/req/states/out/packets")
		reqStateOutTrafficMeter   = metrics.NewMeter("eth/req/states/out/traffic")
		reqReceiptInPacketsMeter  = metrics.NewMeter("eth/req/receipts/in/packets")
		reqReceiptInTrafficMeter  = metrics.NewMeter("eth/req/receipts/in/traffic")
		reqReceiptOutPacketsMeter = metrics.NewMeter("eth/req/receipts/out/packets")
		reqReceiptOutTrafficMeter = metrics.NewMeter("eth/req/receipts/out/traffic")*/
	miscInPacketsMeter  = metrics.NewRegisteredMeter("les/misc/in/packets", nil)
	miscInTrafficMeter  = metrics.NewRegisteredMeter("les/misc/in/traffic", nil)
	miscOutPacketsMeter = metrics.NewRegisteredMeter("les/misc/out/packets", nil)
	miscOutTrafficMeter = metrics.NewRegisteredMeter("les/misc/out/traffic", nil)
)

// meteredMsgReadWriter is a wrapper around a p2p.MsgReadWriter, capable of
// accumulating the above defined metrics based on the data stream contents.
type meteredMsgReadWriter struct {
	p2p.MsgReadWriter     // Wrapped message stream to meter
	version           int // Protocol version to select correct meters
}

// newMeteredMsgWriter wraps a p2p MsgReadWriter with metering support. If the
// metrics system is disabled, this function returns the original object.
func newMeteredMsgWriter(rw p2p.MsgReadWriter) p2p.MsgReadWriter {
	if !metrics.Enabled {
		return rw
	}
	return &meteredMsgReadWriter{MsgReadWriter: rw}
}

// Init sets the protocol version used by the stream to know which meters to
// increment in case of overlapping message ids between protocol versions.
func (rw *meteredMsgReadWriter) Init(version int) {
	rw.version = version
}

func (rw *meteredMsgReadWriter) ReadMsg() (p2p.Msg, error) {
	// Read the message and short circuit in case of an error
	msg, err := rw.MsgReadWriter.ReadMsg()
	if err != nil {
		return msg, err
	}
	// Account for the data traffic
	packets, traffic := miscInPacketsMeter, miscInTrafficMeter
	packets.Mark(1)
	traffic.Mark(int64(msg.Size))

	return msg, err
}

func (rw *meteredMsgReadWriter) WriteMsg(msg p2p.Msg) error {
	// Account for the data traffic
	packets, traffic := miscOutPacketsMeter, miscOutTrafficMeter
	packets.Mark(1)
	traffic.Mark(int64(msg.Size))

	// Send the packet to the p2p layer
	return rw.MsgReadWriter.WriteMsg(msg)
}
