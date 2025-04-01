package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"time"
)

type User struct {
	ID        int64  `json:"id"`
	ImageURL  string `json:"imageUrl"`
	Time      string `json:"time"`
	Day       string `json:"day"`
	TextInfo  string `json:"textInfo"`
	TrainType string `json:"trainType"`
}

type Swipe struct {
	SwiperID  int64 `json:"swiperId"`
	TargetID  int64 `json:"targetId"`
	IsLike    bool  `json:"isLike"`
	Timestamp int64 `json:"timestamp"`
}

type Match struct {
	User1ID   int64 `json:"user1Id"`
	User2ID   int64 `json:"user2Id"`
	Timestamp int64 `json:"timestamp"`
}

type SwipeRequest struct {
	SwiperID int64 `json:"swiperId"`
	TargetID int64 `json:"targetId"`
	IsLike   bool  `json:"isLike"`
}

type Storage struct {
	Users   []User  `json:"users"`
	Swipes  []Swipe `json:"swipes"`
	Matches []Match `json:"matches"`
}

type Controller struct {
	storage  Storage
	dataFile string
	imageDir string
}

func NewController(dataFile, imageDir string) *Controller {
	c := &Controller{
		dataFile: dataFile,
		imageDir: imageDir,
	}

	if err := os.MkdirAll(imageDir, 0755); err != nil {
		log.Printf("Failed to create image directory: %v", err)
	}

	if err := c.loadData(); err != nil {
		log.Printf("Failed to load data, using defaults: %v", err)

		c.storage = Storage{
			Users: []User{
				{ID: 1, ImageURL: "/images/cat.jpeg", Time: "1000:00", Day: "Пн", TextInfo: "Силовая тренировка", TrainType: "Силовая"},
				{ID: 2, ImageURL: "/images/dog.jpeg", Time: "12:00", Day: "Вт", TextInfo: "Кардио нагрузка", TrainType: "Кардио"},
				{ID: 3, ImageURL: "/images/myles.jpeg", Time: "15:00", Day: "Ср", TextInfo: "Йога для начинающих", TrainType: "Йога"},
			},
		}
	}

	return c
}

func (c *Controller) loadData() error {
	data, err := os.ReadFile(c.dataFile)
	if err != nil {
		return fmt.Errorf("reading data file: %w", err)
	}

	if err := json.Unmarshal(data, &c.storage); err != nil {
		return fmt.Errorf("unmarshaling data: %w", err)
	}

	return nil
}

func (c *Controller) saveData() error {

	data, err := json.MarshalIndent(c.storage, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling data: %w", err)
	}

	if err := os.WriteFile(c.dataFile, data, 0644); err != nil {
		return fmt.Errorf("writing data file: %w", err)
	}

	return nil
}

func (c *Controller) AddProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(10 << 30); err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Image is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	imagePath := filepath.Join(c.imageDir, handler.Filename)
	dst, err := os.Create(imagePath)
	if err != nil {
		http.Error(w, "Failed to save image", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to save image", http.StatusInternalServerError)
		return
	}

	user := User{
		ID:        int64(len(c.storage.Users) + 1),
		ImageURL:  "/images/" + handler.Filename,
		Time:      r.FormValue("time"),
		Day:       r.FormValue("day"),
		TextInfo:  r.FormValue("textInfo"),
		TrainType: r.FormValue("trainType"),
	}

	c.storage.Users = append(c.storage.Users, user)
	if err := c.saveData(); err != nil {
		log.Printf("Failed to save data: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func main() {
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	controller := NewController("data/storage.json", "data/images")

	http.Handle("/images/", http.StripPrefix("/images/",
		http.FileServer(http.Dir(controller.imageDir))))

	http.HandleFunc("/api/users", controller.GetUsers)
	http.HandleFunc("/api/next-user/", controller.GetNextUser)
	http.HandleFunc("/api/swipe", controller.Swipe)
	http.HandleFunc("/api/matches/", controller.GetMatches)
	http.HandleFunc("/api/profiles", controller.AddProfile)

	log.Println("Server starting on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (c *Controller) GetUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(c.storage.Users)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (c *Controller) GetNextUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Path[len("/api/next-user/"):]
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	var id int64
	_, err := fmt.Sscanf(userID, "%d", &id)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	for _, user := range c.storage.Users {
		// if user.ID == id {
		// 	continue
		// }

		swiped := false
		for _, swipe := range c.storage.Swipes {
			if swipe.SwiperID == id && swipe.TargetID == user.ID {
				swiped = true
				break
			}
		}

		if !swiped {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(user)
			return
		}
	}

	http.Error(w, "No users available", http.StatusNotFound)
}

func (c *Controller) GetMatches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Path[len("/api/matches/"):]
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	var id int64
	_, err := fmt.Sscanf(userID, "%d", &id)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var userMatches []Match
	for _, match := range c.storage.Matches {
		if match.User1ID == id || match.User2ID == id {
			userMatches = append(userMatches, match)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(userMatches)
}

func (c *Controller) Swipe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SwipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var swiperExists, targetExists bool
	for _, user := range c.storage.Users {
		if user.ID == req.SwiperID {
			swiperExists = true
		}
		if user.ID == req.TargetID {
			targetExists = true
		}
	}

	if !swiperExists || !targetExists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	c.storage.Swipes = append(c.storage.Swipes, Swipe{
		SwiperID:  req.SwiperID,
		TargetID:  req.TargetID,
		IsLike:    req.IsLike,
		Timestamp: time.Now().UnixMilli(),
	})

	isMatch := false
	if req.IsLike {
		for _, swipe := range c.storage.Swipes {
			if swipe.SwiperID == req.TargetID &&
				swipe.TargetID == req.SwiperID &&
				swipe.IsLike {
				isMatch = true
				break
			}
		}
	}

	if isMatch {
		user1ID := min(req.SwiperID, req.TargetID)
		user2ID := max(req.SwiperID, req.TargetID)

		matchExists := false
		for _, match := range c.storage.Matches {
			if match.User1ID == user1ID && match.User2ID == user2ID {
				matchExists = true
				break
			}
		}

		if !matchExists {
			c.storage.Matches = append(c.storage.Matches, Match{
				User1ID:   user1ID,
				User2ID:   user2ID,
				Timestamp: time.Now().UnixMilli(),
			})
		}
	}

	if err := c.saveData(); err != nil {
		log.Printf("Failed to save data: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"isMatch": isMatch,
	}

	if isMatch {
		var lastMatch Match
		for _, match := range c.storage.Matches {
			if (match.User1ID == req.SwiperID && match.User2ID == req.TargetID) ||
				(match.User1ID == req.TargetID && match.User2ID == req.SwiperID) {
				lastMatch = match
			}
		}
		response["match"] = lastMatch
	} else {
		response["match"] = nil
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
