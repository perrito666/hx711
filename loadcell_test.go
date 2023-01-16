package hx711

import (
	"fmt"
	"math/bits"
	"testing"
)

type counterDataPin struct {
	countH, countL int
	get            []bool
	getIdx         int
}

func (c *counterDataPin) loadBits(u []uint32, reset bool) {
	if reset {
		c.get = []bool{}
		c.getIdx = 0
	}
	for _, dec := range u {
		aByte := make([]bool, 0, 32)
		var i uint64
		dec = bits.Reverse32(dec)
		for i = 1; i <= 1<<31; i = i << 1 {
			aByte = append(aByte, uint64(dec)&i == i)
		}
		c.get = append(c.get, aByte[8:]...)
	}
}

func (c *counterDataPin) reset() {
	c.countL = 0
	c.countH = 0
}

func (c *counterDataPin) Get() bool {
	b := c.get[c.getIdx]
	c.getIdx++
	return b
}

func (c *counterDataPin) High() {
	c.countH++
}

func (c *counterDataPin) Low() {
	c.countL++
}

func TestDevice_Calibrate(t *testing.T) {
	dtp := &counterDataPin{}
	var someBbits []uint32
	for i := 0; i < 20; i++ {
		someBbits = append(someBbits, 500000+uint32(i))
	}
	dtp.loadBits(someBbits, false)
	td := Device{
		sck:               dtp,
		dt:                dtp,
		gain:              Gain128,
		smoothingFactor:   10,
		calibrationFactor: 1,
	}

	v, err := td.Calibrate(495.00)
	if err != nil {
		t.Fatal(err)
	}
	cal1 := 0.9900000000
	// yes, this is a hack but much more understandable and easier that starting shifting bits again.
	if fmt.Sprintf("%.10f", v) != fmt.Sprintf("%.10f", cal1) {
		t.Logf("calibration result expected to be %.10f but is %.10f", cal1, v)
		t.FailNow()
	}
	v, err = td.Calibrate(496.00)
	if err != nil {
		t.Fatal(err)
	}
	cal2 := 1.0020181980
	if fmt.Sprintf("%.10f", v) != fmt.Sprintf("%.10f", cal2) {
		t.Logf("calibration result n2 expected to be %.10f but is %.10f", cal2, v)
		t.FailNow()
	}

	if dtp.countL != dtp.countH || dtp.countL != (2+2*24) {
		t.Logf("Gain is %d but tick was called %d times for High and %d times for Low", Gain128, dtp.countH, dtp.countL)
		t.FailNow()
	}
}

func TestDevice_Read(t *testing.T) {
	for _, g := range []gainLVL{Gain128, Gain64, Gain32} {
		dtp := &counterDataPin{}
		var someBits []uint32
		for i := 0; i < 10; i++ {
			someBits = append(someBits, 50000+uint32(i))
		}
		dtp.loadBits(someBits, false)
		td := Device{
			sck:             dtp,
			dt:              dtp,
			gain:            g,
			smoothingFactor: 10,
		}

		v := td.Read()
		if v != 50008 {
			t.Logf("result expected to be %d but is %d", 50008, v)
			t.FailNow()
		}

		if dtp.countL != dtp.countH || dtp.countL != (10*int(g)+10*24) {
			t.Logf("Gain is %d but tick was called %d times for High and %d times for Low", g, dtp.countH, dtp.countL)
			t.FailNow()
		}
	}
}

func TestDevice_read(t *testing.T) {
	for _, g := range []gainLVL{Gain128, Gain64, Gain32} {
		dtp := &counterDataPin{}
		var bits []uint32
		for i := 0; i < 10; i++ {
			bits = append(bits, 50000+uint32(i))
		}
		dtp.loadBits(bits, false)
		td := Device{
			sck:             dtp,
			dt:              dtp,
			gain:            g,
			smoothingFactor: 10,
		}
		for i := 0; i < 10; i++ {
			v := td.read()
			if v != bits[i] {
				t.Logf("byte %d expected to be %b but is %b", i, bits[i], v)
				t.FailNow()
			}
		}
		if dtp.countL != dtp.countH || dtp.countL != (10*int(g)+10*24) {
			t.Logf("Gain is %d but tick was called %d times for High and %d times for Low", g, dtp.countH, dtp.countL)
			t.FailNow()
		}
	}
}

func TestDevice_setGainAndChannel(t *testing.T) {
	dtp := &counterDataPin{}
	for _, g := range []gainLVL{Gain128, Gain64, Gain32} {
		td := Device{
			sck:  dtp,
			gain: g,
		}
		td.setGainAndChannel()
		if dtp.countL != dtp.countH || dtp.countL != int(g) {
			t.Logf("Gain is %d but tick was called %d times for High and %d times for Low", g, dtp.countH, dtp.countL)
			t.FailNow()
		}
		dtp.reset()
	}
}

func TestDevice_tick(t *testing.T) {
	dtp := &counterDataPin{}
	td := Device{
		sck: dtp,
	}
	td.tick()
	if dtp.countL != dtp.countH || dtp.countL != 1 {
		t.Logf("tick was called %d times for High and %d times for Low, expected these to be 1",
			dtp.countH, dtp.countL)
		t.FailNow()
	}
}

func Test_avg(t *testing.T) {
	// This test is here for completeness, I doubt arithmetics will stop working any time soon.
	var avgNum uint32 = 50
	f := func() uint32 {
		avgNum++
		return avgNum
	}
	result := avg(1000, f)
	if result != 1049 {
		t.Logf("expected avg to be X but got %d", result)
		t.FailNow()
	}
}

func Test_toInt64(t *testing.T) {
	type args struct {
		u uint32
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			name: "10",
			args: args{u: 10},
			want: 10,
		},
		{
			name: "100",
			args: args{u: 100},
			want: 100,
		}, {
			name: "1001",
			args: args{u: 1001},
			want: 1001,
		},
		{
			name: "10000",
			args: args{u: 10000},
			want: 10000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toInt64(tt.args.u); got != tt.want {
				t.Errorf("toInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}
