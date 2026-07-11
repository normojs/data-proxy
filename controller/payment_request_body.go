package controller

import (
	"fmt"
	"io"

	"github.com/QuantumNous/new-api/common"
)

const paymentRequestBodyLimitBytes int64 = 1 << 20

func readPaymentRequestBody(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, paymentRequestBodyLimitBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > paymentRequestBodyLimitBytes {
		return body[:paymentRequestBodyLimitBytes], fmt.Errorf("%w: max=%d", common.ErrRequestBodyTooLarge, paymentRequestBodyLimitBytes)
	}
	return body, nil
}
