package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type User struct {
	Name    string `json:"name"`
	Balance int    `json:"balance"`
}

type Quest struct {
	Name string `json:"name"`
	Cost int    `json:"cost"`
}

func connectToDB(dataSourceName string) *sqlx.DB {
	db, err := sqlx.Connect("postgres", dataSourceName)
	if err != nil {
		log.Fatalln(err)
	}
	return db
}

func main() {
	dsn := os.Getenv("DSN")
	db := connectToDB(dsn)
	defer db.Close()

	r := gin.Default()

	r.POST("/create_user", func(c *gin.Context) { createUser(c, db) })
	r.POST("/create_quest", func(c *gin.Context) { createQuest(c, db) })
	r.POST("/complete", func(c *gin.Context) { completeQuest(c, db) })
	r.GET("/history/:userId", func(c *gin.Context) { getUserHistory(c, db) })
	r.GET("/users", func(c *gin.Context) { getAllUsers(c, db) })
	r.GET("/quests", func(c *gin.Context) { getAllQuests(c, db) })

	r.Run()
}

func createUser(c *gin.Context, db *sqlx.DB) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var exists bool
	err := db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM users WHERE name=$1)", user.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking user existence."})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User with this name already exists."})
		return
	}

	if user.Balance == 0 {
		user.Balance = 0
	}

	_, err = db.Exec("INSERT INTO users (name, balance) VALUES ($1, $2)", user.Name, user.Balance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User created successfully."})
}

func createQuest(c *gin.Context, db *sqlx.DB) {
	var quest Quest
	if err := c.ShouldBindJSON(&quest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var exists bool
	err := db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM quests WHERE name=$1)", quest.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking quest existence."})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Quest with this name already exists."})
		return
	}

	_, err = db.Exec("INSERT INTO quests (name, cost) VALUES ($1, $2)", quest.Name, quest.Cost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Quest created successfully."})
}

func completeQuest(c *gin.Context, db *sqlx.DB) {
	var completion struct {
		UserID  int `json:"user_id"`
		QuestID int `json:"quest_id"`
	}

	if err := c.ShouldBindJSON(&completion); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var exists bool
	err := db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM user_quests WHERE user_id=$1 AND quest_id=$2)", completion.UserID, completion.QuestID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking quest completion."})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Quest already completed by user."})
		return
	}

	var cost int
	err = db.Get(&cost, "SELECT cost FROM quests WHERE id=$1", completion.QuestID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Quest not found."})
		return
	}

	tx, err := db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction start failed."})
		return
	}

	_, err = tx.Exec("UPDATE users SET balance = balance + $1 WHERE id = $2", cost, completion.UserID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Updating user balance failed."})
		return
	}

	_, err = tx.Exec("INSERT INTO user_quests (user_id, quest_id) VALUES ($1, $2)", completion.UserID, completion.QuestID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Logging quest completion failed."})
		return
	}

	err = tx.Commit()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Committing transaction failed."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Quest completed successfully."})
}

func getUserHistory(c *gin.Context, db *sqlx.DB) {
	userId := c.Param("userId")

	var user User
	err := db.Get(&user, "SELECT name, balance FROM users WHERE id=$1", userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found."})
		return
	}

	var quests []Quest
	query := "SELECT quests.name, quests.cost FROM quests JOIN user_quests ON quests.id = user_quests.quest_id WHERE user_quests.user_id=$1"
	err = db.Select(&quests, query, userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving user quests."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":   user,
		"quests": quests,
	})
}

func getAllUsers(c *gin.Context, db *sqlx.DB) {
	var users []struct {
		ID      int    `db:"id"`
		Name    string `db:"name"`
		Balance int    `db:"balance"`
	}

	err := db.Select(&users, "SELECT id, name, balance FROM users")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving users."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
	})
}

func getAllQuests(c *gin.Context, db *sqlx.DB) {
	var quests []struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
		Cost int    `db:"cost"`
	}

	err := db.Select(&quests, "SELECT id, name, cost FROM quests")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving quests."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"quests": quests,
	})
}
