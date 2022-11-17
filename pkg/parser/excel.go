package parser

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"io"
)

func ParseXlsx(r io.Reader) error {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return fmt.Errorf("ParseXlsx failed: %w", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			//x.logger.Panic("", zap.Error(err)) что то с этим сделать
		}
	}()

	_, err = f.GetRows("Лист1")
	if err != nil {
		return fmt.Errorf("ParseXlsx failed: %w", err)
	}
	//fmt.Println(rows)

	return nil
}
