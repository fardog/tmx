package tmx

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

func decodeB64LayerData(b []byte) ([]uint32, error) {
	if len(b)%4 != 0 {
		return nil, fmt.Errorf(
			"expected byte array to be divisible by 4, length was %v",
			len(b),
		)
	}

	var uis []uint32
	for i := 0; i < len(b); i = i + 4 {
		ui := binary.LittleEndian.Uint32(b[i : i+4])
		uis = append(uis, ui)
	}

	return uis, nil
}

func decodeCSVLayerData(b []byte) ([]uint32, error) {
	strs := strings.Split(string(b), ",")

	var uis []uint32
	for _, s := range strs {
		ui, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
		if err != nil {
			return nil, err
		}

		uis = append(uis, uint32(ui))
	}

	return uis, nil
}
