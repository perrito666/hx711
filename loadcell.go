package hx711

import (
	"fmt"
	"sync"
	"time"
)

// SCK represents a pin set as out, this is satisfied by a machine.D# pin definition in tinyGo
// before using you should invoke machine.D#.Configure(machine.PinConfig{Mode: machine.PinOutput})
type SCK interface {
	High()
	Low()
}

// DT represents a pin set as in, this is satisfied by a machine.D# pin definition in tinyGo
// before using you should invoke machine.D#.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
// CPP code indicates this is not safe in some Espressif boards and you should use machine.PinInputPulldown instead.
type DT interface {
	Get() bool
}

type gainLVL int

const (
	Gain128 gainLVL = 1 // channel A, gain factor 128
	Gain64  gainLVL = 2 // channel A, gain factor 64
	Gain32  gainLVL = 3 //  channel B, gain factor 32
)

// Device represents a hx711 with a load cell hooked.
// I recommend that you power off between flashes if you use this device as reset causes weird states.
type Device struct {
	// offset holds the zero offset, most devices will have extra weight from plates and install conditions
	offset int64
	// tare, a number on top of offset to zero measures in.
	tare int64
	// sck is the clock pin
	sck SCK
	// dt is the data pin
	dt DT
	// gain is to select the gain between 128, 64 and 32, represented here from 1 to 3
	gain gainLVL
	// smoothingFactor is the amount of reads to average to get a value
	smoothingFactor int
	// calibrationFactor will be used to adjust measures based on known samples
	calibrationFactor float64
	// we want to lock on consecutive read operations to avoid contention
	opMutex sync.Mutex
}

func toInt64(u uint32) int64 {
	return int64(int32(u<<8)) >> 8
}

func avg(times int, f func() uint32) uint32 {
	var r uint32
	for i := 0; i < times; i++ {
		rr := f()
		pr := r
		r += rr
		if i == 0 {
			continue
		}
		// this is a burst of N reads, if the two consecutive reads are too dissimilar we discard it as an outlier
		// which at least in my chip happens a lot.
		if (rr - pr) > 100 {
			r = pr
			continue
		}
		r = r / 2
	}
	return r
}

// New returns a device configured and initialized with the passed ports
// if the device is not appropriately connected this might hang
func New(sck SCK, dt DT, gain gainLVL, smoothingFactor int, settlingWait int) *Device {
	d := &Device{sck: sck, dt: dt, smoothingFactor: smoothingFactor, calibrationFactor: 1}
	d.SetGainAndChannel(gain)
	if settlingWait > 0 {
		time.Sleep(time.Duration(settlingWait) * time.Millisecond)
	}
	// subsequent setting of gain happens in the read
	d.setGainAndChannel()
	for {
		if !d.dt.Get() {
			break
		}
	}
	// make a first read to get a baseline
	d.offset = toInt64(avg(smoothingFactor, d.read))
	return d
}

// tick "ticks" the clock.
// the sleep is for cases where the processor is too fast.
func (d *Device) tick() {
	d.sck.High()
	time.Sleep(time.Microsecond)
	d.sck.Low()
	time.Sleep(time.Microsecond)
}

func (d *Device) SetGainAndChannel(g gainLVL) {
	if g < Gain128 || g > Gain32 {
		g = Gain128
	}
	d.gain = g
}

// setGainAndChannel sets channel and gain when called between reads,I believe it should be called before each read
func (d *Device) setGainAndChannel() {
	for i := 0; i < int(d.gain); i++ {
		d.tick()
	}
}

// read performs a simple read of 24 bits
func (d *Device) read() uint32 {
	var value uint32
	for i := 0; i < 24; i++ {
		d.tick()
		value = value << 1
		if d.dt.Get() {
			value = value | 1
		}
	}
	d.setGainAndChannel()
	return value
}

// Read performs avg of <SmoothingFactor> reads and returns that, adjusted for offset and tare.
func (d *Device) Read() int64 {
	d.opMutex.Lock()
	defer d.opMutex.Unlock()
	return toInt64(avg(d.smoothingFactor, d.read)) - d.offset - d.tare
}

// Tare performs ... well.. tare? https://en.wikipedia.org/wiki/Tare_weight
func (d *Device) Tare() {
	d.opMutex.Lock()
	defer d.opMutex.Unlock()
	d.tare = toInt64(avg(d.smoothingFactor, d.read)) - d.offset
	if d.tare < 0 { // this was a tare on a small value
		d.tare = 0
	}
}

// Zero re-sets offset and tare for the load cell.
func (d *Device) Zero() {
	d.opMutex.Lock()
	defer d.opMutex.Unlock()
	d.offset = toInt64(avg(d.smoothingFactor, d.read))
	d.tare = 0
}

// Calibration is taken from https://github.com/olkal/HX711_ADC

// GetCalibrationFactor returns the factor by which results are multiplied to fine tune weight.
func (d *Device) GetCalibrationFactor() float64 {
	d.opMutex.Lock()
	defer d.opMutex.Unlock()
	return d.calibrationFactor
}

// SetCalibrationFactor sets a number by which reads will be multiplied to obtain a more accurate weight
func (d *Device) SetCalibrationFactor(factor float64) {
	d.opMutex.Lock()
	defer d.opMutex.Unlock()
	d.setCalibrationFactor(factor)
}

func (d *Device) setCalibrationFactor(factor float64) {
	d.calibrationFactor = factor
}

// Calibrate takes the known correct weight of the current load and calculates a factor to correct for drift.
// It is recommended that you save this value once and set it on each ue of a new Device instance for a given
// hardware to avoid having to perform the calibration again.
// Performing this process with various weights improves accuracy.... supposedly, depends on the quality of the cell.
func (d *Device) Calibrate(weightInGrams float64) (float64, error) {
	d.opMutex.Lock()
	defer d.opMutex.Unlock()
	if weightInGrams == 0 {
		return 0, fmt.Errorf("weight needs to be > 0")
	}
	weight := weightInGrams * 1000
	newCF := (float64(toInt64(d.read())) * d.calibrationFactor) / weight
	if newCF == 0 {
		return 0, fmt.Errorf("resulting calibration factor would be 0")
	}
	d.setCalibrationFactor(newCF / 1)
	return d.calibrationFactor, nil
}
