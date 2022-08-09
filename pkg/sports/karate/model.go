package karate

import "github.com/jackc/pgtype"

type Participant struct {
	FullName   string           `json:"full_name"`
	Sex        string           `json:"sex"`
	Age        uint8            `json:"age"`
	Kyi        uint8            `json:"kyū"`
	Dan        uint8            `json:"dan"`
	City       string           `json:"city"`
	KataKumite [2]bool          `json:"kata_kumite"`
	KataGroup  bool             `json:"kata_group"`
	Weight     float32          `json:"weight"`
	Category   pgtype.Int4range `json:"category"` // Пока так, потом подумать как лучше
	Coach      string           `json:"coach"`
}
