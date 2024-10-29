package dmx

import (
	"fmt"

	"github.com/ziutek/ftdi"
)

const (
	vendorID  = 0x0403
	productID = 0x6001
	baudRate  = 250000
)

type Device struct {
	dev   *ftdi.Device
	frame []byte
}

func OpenDevice() (*Device, error) {
	dev, err := ftdi.OpenFirst(vendorID, productID, ftdi.ChannelAny)
	if err != nil {
		return nil, fmt.Errorf("could not open ftdi device: %w", err)
	}

	if err := dev.Reset(); err != nil {
		return nil, fmt.Errorf("could not reset ftdi device: %w", err)
	}

	if err := dev.SetBaudrate(baudRate); err != nil {
		return nil, fmt.Errorf("could not set baud rate for ftdi device: %w", err)
	}

	if err := dev.SetLineProperties(ftdi.DataBits8, ftdi.StopBits2, ftdi.ParityNone); err != nil {
		return nil, fmt.Errorf("could not set baud rate for ftdi device: %w", err)
	}

	if err := dev.SetFlowControl(ftdi.FlowCtrlDisable); err != nil {
		return nil, fmt.Errorf("could not set flow control for ftdi device: %w", err)
	}

	return &Device{
		dev:   dev,
		frame: make([]byte, 511),
	}, nil
}

func (d *Device) Close() error {
	return d.dev.Close()
}

func (d *Device) SetChannel(id int, value byte) {
	d.frame[id] = value
}

func (d *Device) Render() error {
	if err := d.dev.SetLineProperties2(ftdi.DataBits8, ftdi.StopBits2, ftdi.ParityNone, ftdi.BreakOn); err != nil {
		return fmt.Errorf("could not enable break mode for ftdi device: %w", err)
	}

	if err := d.dev.SetLineProperties2(ftdi.DataBits8, ftdi.StopBits2, ftdi.ParityNone, ftdi.BreakOff); err != nil {
		return fmt.Errorf("could not disable break mode for ftdi device: %w", err)
	}

	if _, err := d.dev.Write(d.frame); err != nil {
		return fmt.Errorf("could not write frame to channel: %w", err)
	}

	return nil
}
