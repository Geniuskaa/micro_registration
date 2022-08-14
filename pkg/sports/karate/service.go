package karate

import (
	"context"
	"errors"
	"fmt"
	"github.com/Geniuskaa/micro_registration/pkg/database"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/lib/pq"
)

type Service struct {
	db         *database.Postgres
	ctx        context.Context
	categories map[string]map[pgtype.Int4range]map[string]*catIdLeaf
}

type competitionCategory struct {
	id         int
	kataKumite string
	sex        string
	age        pgtype.Int4range
	kyi        pgtype.Int4range
	weight     pgtype.Int4range
	groupKata  bool
}

type catIdLeaf struct {
	categoryId int
	weightMap  map[pgtype.Int4range]int
}

type Response struct {
	CountOfFailedRows int
	ErrsOfFailedRows  []error
	AddedParticipants []string
	CountOfAddedParts int
}

func NewService(db *database.Postgres, ctx context.Context) *Service {

	m, err := categoryMapInitializer(db.Pool, ctx)
	if err != nil {
		panic(err)
	}

	return &Service{db: db, ctx: ctx, categories: m}
}

func categoryMapInitializer(pool *pgxpool.Pool, ctx context.Context) (map[string]map[pgtype.Int4range]map[string]*catIdLeaf, error) {
	rows, err := pool.Query(ctx, `select id, kata_or_kumite, sex, age, kyi, weight, group_kata from karate_category;`)
	if err != nil {
		return nil, fmt.Errorf("categoryMapInitializer failed: %w", err)
	}

	defer rows.Close()

	m := make(map[string]map[pgtype.Int4range]map[string]*catIdLeaf, 2)
	m["кат"] = make(map[pgtype.Int4range]map[string]*catIdLeaf, 6)
	m["кум"] = make(map[pgtype.Int4range]map[string]*catIdLeaf, 6)

	for rows.Next() {
		compCat := &competitionCategory{}

		err := rows.Scan(&compCat.id, &compCat.kataKumite, &compCat.sex, &compCat.age, &compCat.kyi, &compCat.weight, &compCat.groupKata)
		if err != nil {
			return nil, fmt.Errorf("categoryMapInitializer failed: %w", err)
		}

		switch compCat.kataKumite {
		case "кат":
			v, ok := m["кат"][compCat.age]
			if ok == false {
				m["кат"][compCat.age] = make(map[string]*catIdLeaf, 3)
				m["кат"][compCat.age][compCat.sex] = &catIdLeaf{categoryId: compCat.id}
			} else {
				v[compCat.sex] = &catIdLeaf{categoryId: compCat.id}
			}
		case "кум":
			v, ok := m["кум"][compCat.age]
			if ok == false {
				m["кум"][compCat.age] = make(map[string]*catIdLeaf, 2)

				_, exists := m["кум"][compCat.age][compCat.sex]
				if exists == false {
					m["кум"][compCat.age][compCat.sex] = &catIdLeaf{weightMap: make(map[pgtype.Int4range]int, 6)}
					m["кум"][compCat.age][compCat.sex].weightMap[compCat.weight] = compCat.id
				}
				// Скорее всего это уже не нужно, но если что пусть первые 3 продакшен версии будет тут
				//leave.weightMap[compCat.weight] = compCat.id

			} else {
				leave, exists := v[compCat.sex]
				if exists == false {
					v[compCat.sex] = &catIdLeaf{weightMap: make(map[pgtype.Int4range]int, 6)}
					v[compCat.sex].weightMap[compCat.weight] = compCat.id
				} else {
					leave.weightMap[compCat.weight] = compCat.id
				}
			}
		}

	}

	return m, nil
}

func (s *Service) UploadParticipants(m map[string]interface{}, uuid string) (*Response, error) {

	temp := s.db.Pool.QueryRow(s.ctx, `SELECT id from competition where uuid = $1`, uuid)
	var competId int64
	err := temp.Scan(&competId)
	if err != nil {
		return nil, fmt.Errorf("Competition id cast to int64 failed: %w", err)
	}

	resp := Response{CountOfFailedRows: 0, ErrsOfFailedRows: make([]error, 0, len(m)), AddedParticipants: make([]string, 0, len(m)), CountOfAddedParts: 0}
	rows := make([]pgx.Row, 0, len(m))

	for _, v := range m {
		p := v.(Participant)

		ids := make([]int, 0, 3)

		// Всё, что тут происходит - не логгируется, в конце мы лишь запишем имена тех, кого успешно добавили и напишем
		// сколько человек было с ошибкой.
		if p.KataKumite[0] {
			if p.KataKumite[1] && p.KataGroup { // ката + ката группа + кумите
				// categories[] = {kata: true/false, kataGroup: true/false, kumite: true/false}
				err = s.getCategoryIds(&ids, &p, [3]bool{true, true, true})
				if err != nil {
					resp.ErrsOfFailedRows = append(resp.ErrsOfFailedRows, err)
					resp.CountOfFailedRows++
					continue
				}

			} else if p.KataGroup { // ката + ката группа
				// categories[] = {kata: true/false, kataGroup: true/false, kumite: true/false}
				err = s.getCategoryIds(&ids, &p, [3]bool{true, true, false})
				if err != nil {
					resp.ErrsOfFailedRows = append(resp.ErrsOfFailedRows, err)
					resp.CountOfFailedRows++
					continue
				}
			} else if p.KataKumite[1] { // ката + кумите
				// categories[] = {kata: true/false, kataGroup: true/false, kumite: true/false}
				err = s.getCategoryIds(&ids, &p, [3]bool{true, false, true})
				if err != nil {
					resp.ErrsOfFailedRows = append(resp.ErrsOfFailedRows, err)
					resp.CountOfFailedRows++
					continue
				}
			} else { // только ката
				// categories[] = {kata: true/false, kataGroup: true/false, kumite: true/false}
				err = s.getCategoryIds(&ids, &p, [3]bool{true, false, false})
				if err != nil {
					resp.ErrsOfFailedRows = append(resp.ErrsOfFailedRows, err)
					resp.CountOfFailedRows++
					continue
				}
			}
		} else { //только кумите
			// categories[] = {kata: true/false, kataGroup: true/false, kumite: true/false}
			err = s.getCategoryIds(&ids, &p, [3]bool{false, false, true})
			if err != nil {
				resp.ErrsOfFailedRows = append(resp.ErrsOfFailedRows, err)
				resp.CountOfFailedRows++
				continue
			}
		}

		row := s.db.Pool.QueryRow(s.ctx, `insert into karate_participant (fullname, age, weight, kyi, dan, city, coach_fullname, 
					competition_id, karate_category_ids) values ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning karate_participant.fullname;`,
			p.FullName, p.Age, p.Weight, p.Kyi, p.Dan, p.City, p.Coach, competId, pq.Array(ids))
		rows = append(rows, row)
	}

	for _, row := range rows {
		var uploadedPart string

		err := row.Scan(&uploadedPart)
		if err != nil {
			resp.ErrsOfFailedRows = append(resp.ErrsOfFailedRows, err)
			continue
		}

		resp.AddedParticipants = append(resp.AddedParticipants, uploadedPart)
	}

	resp.CountOfAddedParts = len(resp.AddedParticipants)

	if resp.CountOfFailedRows < (len(m) - resp.CountOfAddedParts) {
		resp.CountOfFailedRows = len(m) - resp.CountOfAddedParts
	}

	return &resp, nil
}

func (s *Service) idFinderKata(p *Participant) (int, error) {
	ageRange := pgtype.Int4range{}
	var err error
	switch p.Age {
	case 10, 11:
		err = ageRange.Set("[10,12)")
	case 12, 13:
		err = ageRange.Set("[12,14)")
	case 14, 15:
		err = ageRange.Set("[14,16)")
	case 16, 17:
		err = ageRange.Set("[16,18)")
	default:
		err = ageRange.Set("[18,)")

	}
	if err != nil {
		return 0, fmt.Errorf("idFinderKata failed: %w", err)
	}

	if p.Age == 10 || p.Age == 11 {
		id := s.categories["кат"][ageRange]["о"].categoryId
		return id, nil
	}

	id := s.categories["кат"][ageRange][p.Sex].categoryId
	return id, nil
}

func (s *Service) idFinderGroupKata(p *Participant) (int, error) {
	ageRange := pgtype.Int4range{}
	var err error
	switch p.Age {
	case 12, 13:
		err = ageRange.Set("[12,14)")
	case 14, 15:
		err = ageRange.Set("[14,16)")
	case 16, 17:
		err = ageRange.Set("[16,18)")
	default:
		err = ageRange.Set("[18,)")

	}
	if err != nil {
		return 0, fmt.Errorf("idFinderGroupKata failed: %w", err)
	}

	id := s.categories["кат"][ageRange]["о"].categoryId
	return id, nil
}

func (s *Service) idFinderKumite(p *Participant) (int, error) {
	if !(p.Category.Upper.Int == 0) && (int32(p.Weight) > p.Category.Upper.Int || int32(p.Weight) < p.Category.Lower.Int) {
		fmt.Println(p.FullName, " ", p.Category.Upper.Int)
		return 0, errors.New("idFinderKumite: weight of participant is out his category")
	}

	ageRange := pgtype.Int4range{}
	var err error
	switch p.Age {
	case 12, 13:
		err = ageRange.Set("[12,14)")
	case 14, 15:
		err = ageRange.Set("[14,16)")
	case 16, 17:
		err = ageRange.Set("[16,18)")
	default:
		err = ageRange.Set("[18,)")

	}
	if err != nil {
		return 0, fmt.Errorf("idFinderKumite failed: %w", err)
	}

	id := s.categories["кум"][ageRange][p.Sex].weightMap[p.Category]

	return id, nil
}

func (s *Service) getCategoryIds(ids *[]int, p *Participant, categories [3]bool) error {
	// categories[] = {kata: true/false, kataGroup: true/false, kumite: true/false}
	if categories[0] {
		id, err := s.idFinderKata(p)
		if err != nil {
			return fmt.Errorf("getCategoryIds failed: %w", err)
		}

		*ids = append(*ids, id)
	}

	if categories[1] {
		id, err := s.idFinderGroupKata(p)
		if err != nil {
			return fmt.Errorf("getCategoryIds failed: %w", err)
		}

		*ids = append(*ids, id)
	}

	if categories[2] {
		id, err := s.idFinderKumite(p)
		if err != nil {
			return fmt.Errorf("getCategoryIds failed: %w", err)
		}

		*ids = append(*ids, id)
	}

	return nil
}
