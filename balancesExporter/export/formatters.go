package export

import (
	"strings"

	"github.com/ElrondNetwork/elrond-go-core/core/pubkeyConverter"
)

const (
	FormatterNamePlainText   = "plain-text"
	FormatterNamePlainJson   = "plain-json"
	FormatterNameRosettaJson = "rosetta-json"
	fourSpaces               = "    "
	addressLength            = 32
)

var (
	AllFormattersNames  = strings.Join([]string{FormatterNamePlainText, FormatterNamePlainText, FormatterNameRosettaJson}, ", ")
	addressConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(addressLength, log)
)

type formatterArgs struct {
	currency         string
	currencyDecimals uint
}
