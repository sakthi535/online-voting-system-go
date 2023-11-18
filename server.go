package main

import (
	"crypto/aes"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type User struct {
	Username string
	Password string
}

type Vote struct {
	UserId      string
	CandidateId string
	Timestamp   string
}

var (
	host     = ""
	port     = 5432
	user     = ""
	password = ""
	dbname   = ""
)

func connect() *sql.DB {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	fmt.Println(psqlInfo)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	return db
}

func runQuery(query string) *sql.Rows {
	db := connect()

	rows, err := db.Query(query)
	if err != nil {
		panic(err)
	}

	return rows
}

func checkError(err error) {
	fmt.Println(string(err.Error()))
	panic(err)
}

func EncryptAES(plaintext string) string {

	key := []byte(os.Getenv("key"))

	bc, err := aes.NewCipher([]byte(key))
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("The block size is %d\n", bc.BlockSize())

	var dst = make([]byte, 16)
	bc.Encrypt(dst, []byte(plaintext))

	bc.Encrypt(dst, []byte(plaintext))

	res := hex.EncodeToString(dst)
	return res
}

func DecryptAES(ct string) string {

	ciphertext_first, _ := hex.DecodeString(ct)
	key := (os.Getenv("key"))

	bc, err := aes.NewCipher([]byte(key))
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("The block size is %d\n", bc.BlockSize())

	pt := make([]byte, len(ciphertext_first))

	bc.Decrypt(pt, ciphertext_first)
	res := string(pt[:])

	return res
}

func encryptUUID(s string) string {
	// n:= len(s)
	return EncryptAES(s[:16]) + EncryptAES(s[16:32]) + EncryptAES(s[20:36])
}

func decryptUUID(s string) string {
	n := len(s)
	return DecryptAES(s[:n/3]) + DecryptAES(s[n/3:2*n/3]) + DecryptAES(s[2*n/3:])[12:]
}

func authenticate(c *gin.Context) {
	var user User

	if err := c.BindJSON(&user); err != nil {
		return
	}

	query := fmt.Sprintf("select userId from voter where username = '%s' and password = '%s';", user.Username, user.Password)
	fmt.Println(query)
	rows := runQuery(query)

	if rows.Next() {
		var userId string
		rows.Scan(&userId)

		userid := encryptUUID(userId)

		responseContent := struct {
			User_Id string "json:'userId'"
		}{
			User_Id: userid,
		}
		c.IndentedJSON(http.StatusCreated, responseContent)
		return
	}
	c.IndentedJSON(http.StatusBadRequest, "User not found")

}

func addVote(c *gin.Context) {

	var vote Vote

	if err := c.BindJSON(&vote); err != nil {
		fmt.Println(err)
		return
	}

	user := decryptUUID(vote.UserId)

	userid, err := uuid.FromString(user)
	candidate, err := uuid.FromString(vote.CandidateId)

	fmt.Println(err)

	query := fmt.Sprintf("insert into votes(userId, candidateId, vote_timestamp) values('%s', '%s', '%s');", userid, candidate, vote.Timestamp)
	runQuery(query)

	c.IndentedJSON(http.StatusCreated, "inserted, ok")
}

func getVote(c *gin.Context) {

	query := fmt.Sprintf("select count(userId) from votes group by candidateId;")
	rows := runQuery(query)

	idx := 0
	votes := make([]int, 2)

	for rows.Next() {
		rows.Scan(&votes[idx])
		idx = idx + 1
	}
	c.IndentedJSON(http.StatusCreated, &votes)

}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func main() {

	err := godotenv.Load("/opt/render/project/go")

	if err != nil {
		panic(err)
	}

	host = os.Getenv("host")
	user = os.Getenv("user")
	password = os.Getenv("password")
	dbname = os.Getenv("dbname")

	router := gin.Default()

	router.Use(corsMiddleware())

	router.POST("/login", authenticate)
	router.POST("/vote", addVote)
	router.GET("/vote", getVote)

	router.Run("0.0.0.0:" + os.Getenv("port_go"))
}
