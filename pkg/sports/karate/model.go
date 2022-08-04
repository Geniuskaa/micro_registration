package karate

type Participant struct {
	FullName   string  `json:"full_name"`
	Sex        string  `json:"sex"`
	Age        uint8   `json:"age"`
	Kyi        uint8   `json:"kyū"`
	City       string  `json:"city"`
	KataKumite [2]bool `json:"kata_kumite"`
	Weight     float32 `json:"weight"`
	Category   uint8   `json:"category"` // Пока так, потом подумать как лучше
	Coach      string  `json:"coach"`
}
