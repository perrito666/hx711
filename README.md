# hx711 LoadCell helpers for [tinygo](https://tinygo.org/)


This is a set of helpers to use a Load cell through a hx711 using tinygo, 
even though according to most device docs and C codes available indicate this
should work in any version of these devices, I only tried it in one.

Special thanks to:
* [this repository](https://github.com/bogde/HX711) who made the C++ version I based this code on.
* [this repository](https://github.com/olkal/HX711_ADC) who made the original calibration i based this on.

## Usage

```go
package main

import (
	"tinygo.perri.to/hx711"
	"machine"
)

// Instantiate a device, this should be thread safe but don't push it
// 100 is a good smoothing factor, it is the number of reads that will be made in raw per Read call
// and averaged.
dev := hx711.New(machine.D4, machine.D5, hx711.Gain128, 100, 400) // in millis, is a good time to wait for the device

// the device is ready to use but i recommend calibrating:
// Once the device has been instantiated (that is a blocking call)
// Put a known weight and make a call to
dev.Calibrate(100.10) // weight in grams

// if you do this with multiple weights multiple times it should be more accurate.

// Finally get a read
weight := dev.Read()
fmt.Printf("whatever is on the scale is %d milligrams", weight)

```

The results are in centigrams, I assume this is true for all models, I only own one hx711 with a 1kg load cell.