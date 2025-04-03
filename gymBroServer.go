// zen-coding

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type User struct {
	FirebaseUID string `json:"firebaseUid"`
	Name        string `json:"name"`
	ImageURL    string `json:"imageUrl"`
	Time        string `json:"time"`
	Day         string `json:"day"`
	TextInfo    string `json:"textInfo"`
	TrainType   string `json:"trainType"`
	Contact     string `json:"contact"`
}

type Swipe struct {
	SwiperID string `json:"swiperId"`
	TargetID string `json:"targetId"`
	IsLike   bool   `json:"isLike"`
}

type Match struct {
	User1ID string `json:"user1Id"`
	User2ID string `json:"user2Id"`
}

type SwipeRequest struct {
	SwiperID string `json:"swiperId"`
	TargetID string `json:"targetId"`
	IsLike   bool   `json:"isLike"`
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
				{
					FirebaseUID: "firebase_uid_AAAAACAT",
					Name:        "KOT",
					ImageURL:    "/images/cat.jpeg",
					Time:        "10:00",
					Day:         "Пн",
					TextInfo:    "Силовая тренировка",
					TrainType:   "Силовая",
					Contact:     "tg: yungeiren",
				},
				{
					FirebaseUID: "firebase_uid_AAAADOG",
					Name:        "DOG",
					ImageURL:    "/images/dog.jpeg",
					Time:        "12:00",
					Day:         "Вт",
					TextInfo:    "Кардио нагрузка",
					TrainType:   "Кардио",
					Contact:     "tg: yungeiren",
				},
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

	firebaseUID := r.FormValue("firebaseUid")
	if firebaseUID == "" {
		http.Error(w, "Firebase UID is required", http.StatusBadRequest)
		return
	}

	var user User
	var imageUpdated bool

	if file, handler, err := r.FormFile("image"); err == nil {
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

		user.ImageURL = "/images/" + handler.Filename
		imageUpdated = true
	}

	user.FirebaseUID = firebaseUID
	user.Name = r.FormValue("name")
	user.Time = r.FormValue("time")
	user.Day = r.FormValue("day")
	user.TextInfo = r.FormValue("textInfo")
	user.TrainType = r.FormValue("trainType")
	user.Contact = r.FormValue("contact")

	found := false
	for i, u := range c.storage.Users {
		if u.FirebaseUID == firebaseUID {
			if imageUpdated {
				c.storage.Users[i].ImageURL = user.ImageURL
			}
			c.storage.Users[i].Name = user.Name
			c.storage.Users[i].Time = user.Time
			c.storage.Users[i].Day = user.Day
			c.storage.Users[i].TextInfo = user.TextInfo
			c.storage.Users[i].TrainType = user.TrainType
			c.storage.Users[i].Contact = user.Contact
			user = c.storage.Users[i]
			found = true
			break
		}
	}

	if !found {
		if !imageUpdated {
			user.ImageURL = "/images/default.jpg"
		}
		c.storage.Users = append(c.storage.Users, user)
	}

	if err := c.saveData(); err != nil {
		log.Printf("Failed to save data: %v", err)
		http.Error(w, "Failed to save data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
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

	userIDStr := r.URL.Path[len("/api/next-user/"):]
	if userIDStr == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	userID := userIDStr

	for _, user := range c.storage.Users {
		if user.FirebaseUID == userID {
			continue
		}

		swiped := false
		for _, swipe := range c.storage.Swipes {
			if swipe.SwiperID == userID && swipe.TargetID == user.FirebaseUID {
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

	userIDStr := r.URL.Path[len("/api/matches/"):]
	if userIDStr == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	var userMatches []Match
	for _, match := range c.storage.Matches {
		if match.User1ID == userIDStr || match.User2ID == userIDStr {
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
		if user.FirebaseUID == req.SwiperID {
			swiperExists = true
		}
		if user.FirebaseUID == req.TargetID {
			targetExists = true
		}
	}

	if !swiperExists || !targetExists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	swipeExists := false
	for _, swipe := range c.storage.Swipes {
		if swipe.SwiperID == req.SwiperID && swipe.TargetID == req.TargetID {
			swipeExists = true
			break
		}
	}

	if req.IsLike && !swipeExists {
		c.storage.Swipes = append(c.storage.Swipes, Swipe{
			SwiperID: req.SwiperID,
			TargetID: req.TargetID,
			IsLike:   req.IsLike,
		})
	}

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
		id1, id2 := req.SwiperID, req.TargetID
		if id1 > id2 {
			id1, id2 = id2, id1
		}

		matchExists := false
		for _, match := range c.storage.Matches {
			if (match.User1ID == id1 && match.User2ID == id2) ||
				(match.User1ID == id2 && match.User2ID == id1) {
				matchExists = true
				break
			}
		}

		if !matchExists {
			c.storage.Matches = append(c.storage.Matches, Match{
				User1ID: id1,
				User2ID: id2,
			})

			if err := c.saveData(); err != nil {
				log.Printf("Failed to save data: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
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
