package repository

import (
	"fmt"
	"strings"
)

type Repository struct {
}

func NewRepository() (*Repository, error) {
	return &Repository{}, nil
}

type PlanetPair struct {
	ID             int
	Planet_start   string
	Planet_end     string
	Distance       int
	Closest_Period int
	Status         string
	Special_info   string
	Like_count     []int
	ImageURL       string
	VideoURL       string
}

func (r *Repository) GetPlanetPairs() ([]PlanetPair, error) {
	orders := []PlanetPair{
		{
			ID:             1,
			Planet_start:   "Земля",
			Planet_end:     "Марс",
			Distance:       215000,
			Closest_Period: 800,
			Status:         "published",
			Special_info:   "Занимательная поездка для всеё семьи! Ни оставит никого равнодушным. Взберитесь на горы олимпа, увидьте землю со стороны и насладитес просторами холодного и безжизненного космоса!",
			Like_count:     []int{101, 102, 103, 1, 2, 3, 4},
			ImageURL:       "Earth-Mars1.png",
			VideoURL:       "Earth-Mars1Vid.mp4",
		},
		{
			ID:             2,
			Planet_start:   "Марс",
			Planet_end:     "Земля",
			Distance:       220000,
			Closest_Period: 860,
			Status:         "draft",
			Special_info:   "aga",
			Like_count:     []int{101, 102, 103, 1, 2, 3, 4},
			ImageURL:       "Mars-Earth2.png",
			VideoURL:       "Mars-Earth2Vid.mp4",
		},
		{
			ID:             3,
			Planet_start:   "Земля",
			Planet_end:     "Венера",
			Distance:       222000,
			Closest_Period: 90,
			Status:         "published",
			Special_info:   "aga",
			Like_count:     []int{101, 102, 111, 1022, 103, 1, 2, 3, 4},
			ImageURL:       "Earth-Venera3.png",
			VideoURL:       "Earth-Venera3Vid.mp4",
		},
		{
			ID:             4,
			Planet_start:   "Венера",
			Planet_end:     "Марс",
			Distance:       300000,
			Closest_Period: 150,
			Status:         "published",
			Special_info:   "aga",
			Like_count:     []int{101, 1, 2, 3, 4},
			ImageURL:       "Venera-Mars4.png",
			VideoURL:       "Venera-Mars4Vid.mp4",
		},
		{
			ID:             5,
			Planet_start:   "Юпитер",
			Planet_end:     "Земля",
			Distance:       500000,
			Closest_Period: 3400,
			Status:         "published",
			Special_info:   "aga",
			Like_count:     []int{101, 40, 34, 111, 102, 103, 1, 2, 3, 4, 5, 6, 7},
			ImageURL:       "Jupiter-Earth5.png",
			VideoURL:       "Jupiter-Earth5Vid.mp4",
		},
		{
			ID:             6,
			Planet_start:   "Юипитер",
			Planet_end:     "Марс",
			Distance:       470500,
			Closest_Period: 55,
			Status:         "published",
			Special_info:   "aga",
			Like_count:     []int{101, 102},
			ImageURL:       "Jupiter-Mars6.png",
			VideoURL:       "Jupiter-Mars6Vid.mp4",
		},
		{
			ID:             7,
			Planet_start:   "Сатурн",
			Planet_end:     "Марс",
			Distance:       200,
			Closest_Period: 20,
			Status:         "deleted",
			Special_info:   "aga",
			Like_count:     []int{101, 102, 103, 1, 2, 3, 4},
			ImageURL:       "Saturn-Mars7.jpeg",
			VideoURL:       "Saturn-Mars7Vid.mp4",
		},
	}

	if len(orders) == 0 {
		return nil, fmt.Errorf("массив пустой")
	}

	return orders, nil
}

func (r *Repository) GetPublishedPlanetPairs() ([]PlanetPair, error) {
	PlanetPairs, err := r.GetPlanetPairs()
	if err != nil {
		return nil, err
	}

	var published []PlanetPair
	for _, PlanetPair := range PlanetPairs {
		if strings.ToLower(PlanetPair.Status) == "published" {
			published = append(published, PlanetPair)
		}
	}
	return published, nil
}

func (r *Repository) GetPlanetPairsByRange(minRange, maxRange int) ([]PlanetPair, error) {
	PlanetPairs, err := r.GetPublishedPlanetPairs()
	if err != nil {
		return nil, err
	}

	var result []PlanetPair
	for _, PlanetPair := range PlanetPairs {
		if PlanetPair.Distance >= minRange && PlanetPair.Distance <= maxRange {
			result = append(result, PlanetPair)
		}
	}
	return result, nil
}

func (r *Repository) GetPlanetPairDraft() (PlanetPair, error) {
	PlanetPairs, err := r.GetPlanetPairs()
	if err != nil {
		return PlanetPair{}, err
	}

	for _, PlanetPair := range PlanetPairs {
		if strings.ToLower(PlanetPair.Status) == "draft" {
			return PlanetPair, nil
		}
	}

	return PlanetPair{}, fmt.Errorf("нет черновика")
}
