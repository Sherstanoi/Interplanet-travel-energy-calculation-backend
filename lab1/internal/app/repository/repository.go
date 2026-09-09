package repository

import (
	"fmt"
	"strconv"
	"strings"
)

type Repository struct {
}

func NewRepository() (*Repository, error) {
	return &Repository{}, nil
}

type Card struct {
	ID             int
	Planet_start   string
	Planet_end     string
	Spaceship_mass int
	Fuel_volume    int
	Status         string
	Special_info   string
	Like_count     int
	ImageURL       string
	VideoURL       string
}

func (r *Repository) GetCards() ([]Card, error) {
	orders := []Card{
		{
			ID:             1,
			Planet_start:   "Земля",
			Planet_end:     "Марс",
			Spaceship_mass: 220,
			Fuel_volume:    2,
			Status:         "published",
			Special_info:   "Занимательная поездка для всеё семьи! Ни оставит никого равнодушным. Взберитесь на горы олимпа, увидьте землю со стороны и насладитес просторами холодного и безжизненного космоса!",
			Like_count:     13570,
			ImageURL:       "Earth-Mars1.png",
			VideoURL:       "Earth-Mars1Vid.mp4",
		},
		{
			ID:             2,
			Planet_start:   "Марс",
			Planet_end:     "Земля",
			Spaceship_mass: 220,
			Fuel_volume:    2,
			Status:         "draft",
			Special_info:   "aga",
			Like_count:     13570,
			ImageURL:       "Mars-Earth2.png",
			VideoURL:       "Mars-Earth2Vid.mp4",
		},
		{
			ID:             3,
			Planet_start:   "Земля",
			Planet_end:     "Венера",
			Spaceship_mass: 10,
			Fuel_volume:    2,
			Status:         "published",
			Special_info:   "aga",
			Like_count:     13570,
			ImageURL:       "Earth-Venera3.png",
			VideoURL:       "Earth-Venera3Vid.mp4",
		},
		{
			ID:             4,
			Planet_start:   "Венера",
			Planet_end:     "Марс",
			Spaceship_mass: 2000,
			Fuel_volume:    2,
			Status:         "published",
			Special_info:   "aga",
			Like_count:     13570,
			ImageURL:       "Venera-Mars4.png",
			VideoURL:       "Venera-Mars4Vid.mp4",
		},
		{
			ID:             5,
			Planet_start:   "Юпитер",
			Planet_end:     "Земля",
			Spaceship_mass: 350,
			Fuel_volume:    2,
			Status:         "published",
			Special_info:   "aga",
			Like_count:     13570,
			ImageURL:       "Jupiter-Earth5.png",
			VideoURL:       "Jupiter-Earth5Vid.mp4",
		},
		{
			ID:             6,
			Planet_start:   "Юипитер",
			Planet_end:     "Марс",
			Spaceship_mass: 30,
			Fuel_volume:    2,
			Status:         "published",
			Special_info:   "aga",
			Like_count:     13570,
			ImageURL:       "Jupiter-Mars6.png",
			VideoURL:       "Jupiter-Mars6Vid.mp4",
		},
		{
			ID:             7,
			Planet_start:   "Сатурн",
			Planet_end:     "Марс",
			Spaceship_mass: 200,
			Fuel_volume:    2,
			Status:         "deleted",
			Special_info:   "aga",
			Like_count:     13570,
			ImageURL:       "Saturn-Mars7.jpeg",
			VideoURL:       "Saturn-Mars7Vid.mp4",
		},
	}

	if len(orders) == 0 {
		return nil, fmt.Errorf("массив пустой")
	}

	return orders, nil
}

func (r *Repository) GetPublishedCards() ([]Card, error) {
	cards, err := r.GetCards()
	if err != nil {
		return nil, err
	}

	var published []Card
	for _, card := range cards {
		if strings.ToLower(card.Status) == "published" {
			published = append(published, card)
		}
	}
	return published, nil
}

func (r *Repository) GetCard(id int) (Card, error) {
	cards, err := r.GetPublishedCards()
	if err != nil {
		return Card{}, err
	}

	for _, card := range cards {
		if card.ID == id {
			return card, nil
		}
	}
	return Card{}, fmt.Errorf("карточка не найдена")
}

func (r *Repository) GetCardsByMass(massStr string) ([]Card, error) {
	cards, err := r.GetPublishedCards()
	if err != nil {
		return nil, err
	}

	var result []Card
	for _, card := range cards {
		if strings.Contains(strconv.Itoa(card.Spaceship_mass), massStr) {
			result = append(result, card)
		}
	}
	return result, nil
}

func (r *Repository) GetDraft() (Card, error) {
	cards, err := r.GetCards()
	if err != nil {
		return Card{}, err
	}

	for _, card := range cards {
		if strings.ToLower(card.Status) == "draft" {
			return card, nil
		}
	}

	return Card{}, fmt.Errorf("нет черновика")
}
