package main

import (
	"time"

	"github.com/waltzofpearls/serial"
)

const (
	SerialPort = "/dev/ttyAMA0"
	SerialBaud = 500000

	UsI2cSpeed       = 10
	UsI2cIdx         = 0
	LegoUsI2cAddr    = 0x02
	LegoUsI2cDataReg = 0x42

	PortA = 0
	PortB = 1
	PortC = 2
	PortD = 3

	Port1 = 0
	Port2 = 1
	Port3 = 2
	Port4 = 3

	MaskD0_M = 0x01
	MaskD1_M = 0x02
	Mask9V   = 0x04
	MaskD0_S = 0x08
	MaskD1_S = 0x10

	ByteMsgType            = 0 // MsgType is the first byte.
	MsgTypeChangeAddr      = 1 // Change the UART address.
	MsgTypeSensorType      = 2 // Change/set the sensor type.
	MsgTypeValues          = 3 // Set the motor speed and direction, and return the sesnors and encoders.
	MsgTypeEStop           = 4 // Float motors immidately
	MsgTypeTimeoutSettings = 5 // Set the timeout
	ByteNewAddress         = 1
	ByteSensor1Type        = 1
	ByteSensor2Type        = 2
	ByteTimeout            = 1

	TypeMotorPwm      = 0
	TypeMotorSpeed    = 1
	TypeMotorPosition = 2

	TypeSensorRaw            = 0 // - 31
	TypeSensorLightOff       = 0
	TypeSensorLightOn        = (MaskD0_M | MaskD0_S)
	TypeSensorTouch          = 32
	TypeSensorUltrasonicCont = 33
	TypeSensorUltrasonicSs   = 34
	TypeSensorRcxLight       = 35 // tested minimally
	TypeSensorColorFull      = 36
	TypeSensorColorRed       = 37
	TypeSensorColorGreen     = 38
	TypeSensorColorBlue      = 39
	TypeSensorColorNone      = 40
	TypeSensorI2c            = 41
	TypeSensorI2c9v          = 42

	TypeSensorEv3UsM0 = 43 // Continuous measurement, distance, cm
	TypeSensorEv3UsM1 = 44 // Continuous measurement, distance, in
	TypeSensorEv3UsM2 = 45 // Listen, 0 r 1 depending on presence of another US sensor
	TypeSensorEv3UsM3 = 46
	TypeSensorEv3UsM4 = 47
	TypeSensorEv3UsM5 = 48
	TypeSensorEv3UsM6 = 49

	TypeSensorEv3ColorM0 = 50 // Reflected
	TypeSensorEv3ColorM1 = 51 // Ambient
	TypeSensorEv3ColorM2 = 52 // Color, min is 0, max is 7 (brown)
	TypeSensorEv3ColorM3 = 53 // Raw reflected
	TypeSensorEv3ColorM4 = 54 // Raw Color Components
	TypeSensorEv3ColorM5 = 55 // Calibration???  Not currently implemented

	TypeSensorEv3GyroM0 = 56 // Angle
	TypeSensorEv3GyroM1 = 57 // Rotational Speed
	TypeSensorEv3GyroM2 = 58 // Raw sensor value ???
	TypeSensorEv3GyroM3 = 59 // Angle and Rotational Speed?
	TypeSensorEv3GyroM4 = 60 // Calibration ???

	TypeSensorEv3InfraredM0 = 61 // Proximity, 0 to 100
	TypeSensorEv3InfraredM1 = 62 // IR Seek, -25 (far left) to 25 (far right)
	TypeSensorEv3InfraredM2 = 63 // IR Remote Control, 0 - 11
	TypeSensorEv3InfraredM3 = 64
	TypeSensorEv3InfraredM4 = 65
	TypeSensorEv3InfraredM5 = 66

	TypeSensorEv3Touch0 = 67

	TypeSensorEv3TouchDebounce = 68 // EV3 Touch sensor, debounced.
	TypeSensorTouchDebounce    = 69 // NXT Touch sensor, debounced.

	ReturnVersion = 70 // Returns firmware version.

	BitI2cMid  = 0x01 // Do one of those funny clock pulses between writing and reading. defined for each device.
	BitI2cSame = 0x02 // The transmit data, and the number of bytes to read and write isn't going to change. defined for each device.

	IndexRed   = 0
	IndexGreen = 1
	IndexBlue  = 2
	IndexBlank = 3
)

var (
	Array         [256]int
	BytesReceived int
	BitOffset     int
	Retried       int
)

type BrickPi struct {
	Serial     *serial.Port
	Address    [2]int
	MotorSpeed [4]uint8

	MotorEnable [4]uint8

	EncoderOffset [4]uint8
	Encoder       [4]uint8

	Sensor         [4]uint8
	SensorArray    [4][4]uint8
	SensorType     [4]int
	SensorSettings [4][8]int

	SensorI2cDevices [4]int
	SensorI2cSpeed   [4]int
	SensorI2cAddr    [4][8]int
	SensorI2cWrite   [4][8]int
	SensorI2cRead    [4][8]int
	SensorI2cOut     [4][8][16]int
	SensorI2cIn      [4][8][16]int
	Timeout          int
}

func New() *BrickPi {
	return &BrickPi{Address: [2]int{1, 2}}
}

func (bp *BrickPi) Setup() error {
	ser, err := serial.OpenPort(&serial.Config{
		Name: SerialPort,
		Baud: SerialBaud,
	})
	if err != nil {
		return err
	}
	time.Sleep(time.Second * 1)
	bp.Serial = ser
	return nil
}

func (bp *BrickPi) SetupSensors() int {
	for i := 0; i < 2; i++ {
		Array = [256]int{}
		BitOffset = 0
		Array[ByteMsgType] = MsgTypeSensorType
		Array[ByteSensor1Type] = bp.SensorType[Port1+i*2]
		Array[ByteSensor2Type] = bp.SensorType[Port2+i*2]
		for ii := 0; ii < 2; ii++ {
			port := i*2 + ii
			if Array[ByteSensor1Type+ii] == TypeSensorUltrasonicCont {
				Array[ByteSensor1Type+ii] = TypeSensorI2c
				bp.SensorI2cSpeed[port] = UsI2cSpeed
				bp.SensorI2cDevices[port] = 1
				bp.SensorSettings[port][UsI2cIdx] = BitI2cMid | BitI2cSame
				bp.SensorI2cAddr[port][UsI2cIdx] = LegoUsI2cAddr
				bp.SensorI2cWrite[port][UsI2cIdx] = 1
				bp.SensorI2cRead[port][UsI2cIdx] = 1
				bp.SensorI2cOut[port][UsI2cIdx][0] = LegoUsI2cDataReg
			}
			if Array[ByteSensor1Type+ii] == TypeSensorI2c || Array[ByteSensor1Type+ii] == TypeSensorI2c9v {
				bp.AddBits(3, 0, 8, bp.SensorI2cSpeed[port])

				if bp.SensorI2cDevices[port] > 8 {
					bp.SensorI2cDevices[port] = 8
				}

				if bp.SensorI2cDevices[port] == 0 {
					bp.SensorI2cDevices[port] = 1
				}

				bp.AddBits(3, 0, 3, (bp.SensorI2cDevices[port] - 1))

				for device := 0; device < bp.SensorI2cDevices[port]; device++ {
					bp.AddBits(3, 0, 7, (bp.SensorI2cAddr[port][device] >> 1))
					bp.AddBits(3, 0, 2, bp.SensorSettings[port][device])
					if bp.SensorSettings[port][device]&BitI2cSame != 0 {
						bp.AddBits(3, 0, 4, bp.SensorI2cWrite[port][device])
						bp.AddBits(3, 0, 4, bp.SensorI2cRead[port][device])

						for outByte := 0; outByte < bp.SensorI2cWrite[port][device]; outByte++ {
							bp.AddBits(3, 0, 8, bp.SensorI2cOut[port][device][outByte])
						}
					}
				}
			}
		}
		txBytes := (((BitOffset + 7) / 8) + 3) // eq to UartTxBytes
		bp.BrickPiTx(bp.Address[i], txBytes, Array)
		res, BytesReceived, InArray := bp.BrickPiRx(5) // Timeout set to 5 seconds to setup EV3 sensors successfully
		if res != 0 {
			return -1
		}
		for i = 0; i < len(InArray); i++ {
			Array[i] = InArray[i]
		}
		if !(BytesReceived == 1 && Array[ByteMsgType] == MsgTypeSensorType) {
			return -1
		}
	}
	return 0
}

func (bp *BrickPi) AddBits(byteOffset, bitOffset, bits, value int) {
}

func (bp *BrickPi) BrickPiTx(dest, ByteCount int, OutArray [256]int) {
}

func (bp *BrickPi) BrickPiRx(timeout int) (int, int, [256]int) {
	var InArray [256]int
	return 0, 0, InArray
}

func main() {
	bp := New()
	bp.Setup()
	bp.SetupSensors()
}
