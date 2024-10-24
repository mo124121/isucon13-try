package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultSessionIDKey      = "SESSIONID"
	defaultSessionExpiresKey = "EXPIRES"
	defaultUserIDKey         = "USERID"
	defaultUsernameKey       = "USERNAME"
	bcryptDefaultCost        = bcrypt.MinCost
)

var fallbackImage = "../img/NoImage.jpg"

type UserModel struct {
	ID             int64  `db:"id"`
	Name           string `db:"name"`
	DisplayName    string `db:"display_name"`
	Description    string `db:"description"`
	HashedPassword string `db:"password"`
}

type User struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Theme       Theme  `json:"theme,omitempty"`
	IconHash    string `json:"icon_hash,omitempty"`
}

type Theme struct {
	ID       int64 `json:"id"`
	DarkMode bool  `json:"dark_mode"`
}

type ThemeModel struct {
	ID       int64 `db:"id"`
	UserID   int64 `db:"user_id"`
	DarkMode bool  `db:"dark_mode"`
}

type PostUserRequest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	// Password is non-hashed password.
	Password string               `json:"password"`
	Theme    PostUserRequestTheme `json:"theme"`
}

type PostUserRequestTheme struct {
	DarkMode bool `json:"dark_mode"`
}

type LoginRequest struct {
	Username string `json:"username"`
	// Password is non-hashed password.
	Password string `json:"password"`
}

type PostIconRequest struct {
	Image []byte `json:"image"`
}

type PostIconResponse struct {
	ID int64 `json:"id"`
}

func getIconHashFromName(username string) (string, error) {
	userDir := fmt.Sprintf("/var/www/icons/%s", username)
	hashPath := filepath.Join(userDir, "icon.hash")
	// ハッシュファイルの読み込み
	iconHashByte, err := os.ReadFile(hashPath)
	if err != nil {
		if os.IsNotExist(err) {
			iconHash, calcErr := calculateFallbackHash()
			if calcErr != nil {
				return "", calcErr
			}
			return iconHash, nil
		}
	}
	iconHash := string(iconHashByte)
	return iconHash, nil
}

func getIconHandler(c echo.Context) error {
	username := c.Param("username")

	// アイコンファイルとハッシュファイルのパス
	userDir := fmt.Sprintf("/var/www/icons/%s", username)
	iconPath := filepath.Join(userDir, "icon")

	// ハッシュ取得
	iconHash, err := getIconHashFromName(username)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get icon hash")
	}

	// If-None-Matchヘッダーを検証
	ifNoneMatch := c.Request().Header.Get("If-None-Match")
	if ifNoneMatch == fmt.Sprintf(`"%s"`, string(iconHash)) {
		// ハッシュが一致する場合は304を返す
		return c.NoContent(http.StatusNotModified)
	}

	// アイコン画像の読み込み
	iconData, err := os.ReadFile(iconPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.File(fallbackImage)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to read icon")
	}

	// ETagにハッシュ値をセットして画像を返す
	c.Response().Header().Set("ETag", fmt.Sprintf(`"%s"`, string(iconHash)))
	return c.Blob(http.StatusOK, "image/jpeg", iconData)
}

const iconDirRoot = "/var/www/icons/"

// ハッシュを計算する関数
func calculateHash(data []byte) string {
	hash := sha256.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

func postIconHandler(c echo.Context) error {
	// ユーザーセッションの確認
	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userName := sess.Values[defaultUsernameKey].(string)

	var req *PostIconRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to decode the request body as json")
	}

	// 画像データをディレクトリに保存する処理

	// 保存先のディレクトリを指定（例: /var/www/icons/userID/）
	userDir := fmt.Sprintf("%s%s", iconDirRoot, userName)
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create user icon directory: "+err.Error())
	}

	// ファイルとして保存
	iconPath := fmt.Sprintf("%s/icon", userDir)
	if err := os.WriteFile(iconPath, req.Image, 0644); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save user icon: "+err.Error())
	}
	// アイコンハッシュを計算して保存
	iconHash := calculateHash(req.Image)
	hashPath := filepath.Join(userDir, "icon.hash")
	if err := os.WriteFile(hashPath, []byte(iconHash), 0644); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save icon hash")
	}

	//更新されたのでUserCacheの削除
	ctx := c.Request().Context()
	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}
	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()
	userModel := UserModel{}
	if err := tx.GetContext(ctx, &userModel, "SELECT * FROM users WHERE name = ?", userName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "not found user that has the given username")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user: "+err.Error())
	}
	userCache.Delete(userModel.ID)

	// 成功した場合のレスポンス
	return c.JSON(http.StatusCreated, &PostIconResponse{
		ID: 1, // 1はダミー うまくいくか謎
	})
}

// アイコンディレクトリのルートパス

// アイコンディレクトリをすべて削除する関数
func deleteAllIconDirs() error {
	// まずは削除対象のディレクトリをリストアップ
	var dirsToDelete []string

	err := filepath.Walk(iconDirRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// ルートディレクトリ以外のすべてのディレクトリをリストアップ
		if info.IsDir() && path != iconDirRoot {
			dirsToDelete = append(dirsToDelete, path)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to list directories: %v", err)
	}

	// リストに基づいてディレクトリを削除
	for _, dir := range dirsToDelete {
		fmt.Println("Removing directory:", dir)
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("failed to remove directory %s: %v", dir, err)
		}
	}

	fmt.Println("All user icon directories have been deleted successfully.")
	return nil
}

func getMeHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	user, err := getUser(ctx, tx, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill user: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, user)
}

// ユーザ登録API
// POST /api/register
func registerHandler(c echo.Context) error {
	ctx := c.Request().Context()
	defer c.Request().Body.Close()

	req := PostUserRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to decode the request body as json")
	}

	if req.Name == "pipe" {
		return echo.NewHTTPError(http.StatusBadRequest, "the username 'pipe' is reserved")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptDefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate hashed password: "+err.Error())
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	userModel := UserModel{
		Name:           req.Name,
		DisplayName:    req.DisplayName,
		Description:    req.Description,
		HashedPassword: string(hashedPassword),
	}

	result, err := tx.NamedExecContext(ctx, "INSERT INTO users (name, display_name, description, password) VALUES(:name, :display_name, :description, :password)", userModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert user: "+err.Error())
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get last inserted user id: "+err.Error())
	}

	userModel.ID = userID

	themeModel := ThemeModel{
		UserID:   userID,
		DarkMode: req.Theme.DarkMode,
	}
	if _, err := tx.NamedExecContext(ctx, "INSERT INTO themes (user_id, dark_mode) VALUES(:user_id, :dark_mode)", themeModel); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert user theme: "+err.Error())
	}

	if out, err := exec.Command("pdnsutil", "add-record", "u.isucon.local", req.Name, "A", "0", powerDNSSubdomainAddress).CombinedOutput(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, string(out)+": "+err.Error())
	}

	user, err := fillUserResponse(ctx, tx, userModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill user: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusCreated, user)
}

// ユーザログインAPI
// POST /api/login
func loginHandler(c echo.Context) error {
	ctx := c.Request().Context()
	defer c.Request().Body.Close()

	req := LoginRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to decode the request body as json")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	userModel := UserModel{}
	// usernameはUNIQUEなので、whereで一意に特定できる
	err = tx.GetContext(ctx, &userModel, "SELECT * FROM users WHERE name = ?", req.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid username or password")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	err = bcrypt.CompareHashAndPassword([]byte(userModel.HashedPassword), []byte(req.Password))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid username or password")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to compare hash and password: "+err.Error())
	}

	sessionEndAt := time.Now().Add(1 * time.Hour)

	sessionID := uuid.NewString()

	sess, err := session.Get(defaultSessionIDKey, c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "failed to get session")
	}

	sess.Options = &sessions.Options{
		Domain: "u.isucon.local",
		MaxAge: int(60000),
		Path:   "/",
	}
	sess.Values[defaultSessionIDKey] = sessionID
	sess.Values[defaultUserIDKey] = userModel.ID
	sess.Values[defaultUsernameKey] = userModel.Name
	sess.Values[defaultSessionExpiresKey] = sessionEndAt.Unix()

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save session: "+err.Error())
	}

	return c.NoContent(http.StatusOK)
}

// ユーザ詳細API
// GET /api/user/:username
func getUserHandler(c echo.Context) error {
	ctx := c.Request().Context()
	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	username := c.Param("username")

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	userModel := UserModel{}
	if err := tx.GetContext(ctx, &userModel, "SELECT * FROM users WHERE name = ?", username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "not found user that has the given username")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user: "+err.Error())
	}

	user, err := fillUserResponse(ctx, tx, userModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill user: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, user)
}

func verifyUserSession(c echo.Context) error {
	sess, err := session.Get(defaultSessionIDKey, c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "failed to get session")
	}

	sessionExpires, ok := sess.Values[defaultSessionExpiresKey]
	if !ok {
		return echo.NewHTTPError(http.StatusForbidden, "failed to get EXPIRES value from session")
	}

	_, ok = sess.Values[defaultUserIDKey].(int64)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "failed to get USERID value from session")
	}

	now := time.Now()
	if now.Unix() > sessionExpires.(int64) {
		return echo.NewHTTPError(http.StatusUnauthorized, "session has expired")
	}

	return nil
}

var fallbackIconHash string
var once sync.Once

func calculateFallbackHash() (string, error) {
	var err error
	once.Do(func() {
		image, readErr := os.ReadFile(fallbackImage)
		if readErr != nil {
			err = readErr
			return
		}
		hash := sha256.Sum256(image)
		fallbackIconHash = fmt.Sprintf("%x", hash)
	})
	return fallbackIconHash, err
}

func fillUserResponse(ctx context.Context, tx *sqlx.Tx, userModel UserModel) (User, error) {
	themeModel := ThemeModel{}
	if err := tx.GetContext(ctx, &themeModel, "SELECT * FROM themes WHERE user_id = ?", userModel.ID); err != nil {
		return User{}, err
	}

	iconHash, err := getIconHashFromName(userModel.Name)
	if err != nil {
		return User{}, err
	}

	user := User{
		ID:          userModel.ID,
		Name:        userModel.Name,
		DisplayName: userModel.DisplayName,
		Description: userModel.Description,
		Theme: Theme{
			ID:       themeModel.ID,
			DarkMode: themeModel.DarkMode,
		},
		IconHash: iconHash,
	}

	return user, nil
}

type UserCache struct {
	mu    sync.Mutex
	items map[int64]User
}

func NewUserCache() *UserCache {
	m := make(map[int64]User)
	c := &UserCache{
		items: m,
	}
	return c
}

func (c *UserCache) Set(userID int64, user User) {
	c.mu.Lock()
	c.items[userID] = user
	c.mu.Unlock()
}

func (c *UserCache) Get(ctx context.Context, tx *sqlx.Tx, userID int64) (User, error) {
	c.mu.Lock()
	v, ok := c.items[userID]
	c.mu.Unlock()
	if ok {
		return v, nil
	}
	v, err := getUserHeavy(ctx, tx, userID)
	if err != nil {
		return User{}, err
	}
	c.Set(userID, v)
	return v, nil
}

func (c *UserCache) Delete(userID int64) {
	c.mu.Lock()
	delete(c.items, userID)
	c.mu.Unlock()
}

func getUser(ctx context.Context, tx *sqlx.Tx, userID int64) (User, error) {
	return userCache.Get(ctx, tx, userID)
}

func getUserHeavy(ctx context.Context, tx *sqlx.Tx, userID int64) (User, error) {

	userModel := UserModel{}
	if err := tx.GetContext(ctx, &userModel, "SELECT * FROM users WHERE id = ?", userID); err != nil {
		return User{}, err
	}

	themeModel := ThemeModel{}
	if err := tx.GetContext(ctx, &themeModel, "SELECT * FROM themes WHERE user_id = ?", userModel.ID); err != nil {
		return User{}, err
	}

	iconHash, err := getIconHashFromName(userModel.Name)
	if err != nil {
		return User{}, err
	}

	user := User{
		ID:          userModel.ID,
		Name:        userModel.Name,
		DisplayName: userModel.DisplayName,
		Description: userModel.Description,
		Theme: Theme{
			ID:       themeModel.ID,
			DarkMode: themeModel.DarkMode,
		},
		IconHash: iconHash,
	}

	return user, nil
}

func getUsers(ctx context.Context, tx *sqlx.Tx, userIDs []int64) ([]User, error) {
	users := make([]User, len(userIDs))
	if len(userIDs) == 0 {
		return users, nil
	}
	var userModels []UserModel
	query, args, err := sqlx.In("SELECT * FROM users WHERE id IN (?)", userIDs)
	if err != nil {
		return nil, err
	}
	if err := tx.SelectContext(ctx, &userModels, tx.Rebind(query), args...); err != nil {
		return nil, err
	}
	// UserID -> UserModelのマッピングを作成
	userMap := make(map[int64]UserModel, len(userModels))
	for _, owner := range userModels {
		userMap[owner.ID] = owner
	}

	// ユーザーのテーマとアイコンをプリロード
	var themeModels []ThemeModel
	query, args, err = sqlx.In("SELECT * FROM themes WHERE user_id IN (?)", userIDs)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to build theme query: "+err.Error())
	}
	if err := tx.SelectContext(ctx, &themeModels, tx.Rebind(query), args...); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to load themes: "+err.Error())
	}
	themeMap := make(map[int64]ThemeModel, len(themeModels))
	for _, theme := range themeModels {
		themeMap[theme.UserID] = theme
	}

	iconMap := make(map[int64]string)

	for id, user := range userMap {
		iconMap[id], err = getIconHashFromName(user.Name)
		if err != nil {
			return make([]User, 0), err
		}
	}

	for i, id := range userIDs {
		userModel := userMap[id]
		themeModel := themeMap[id]
		iconhash := iconMap[id]

		if value, exists := iconMap[id]; exists {
			iconhash = value
		}

		users[i] = User{
			ID:          userModel.ID,
			Name:        userModel.Name,
			DisplayName: userModel.DisplayName,
			Description: userModel.Description,
			Theme: Theme{
				ID:       themeModel.ID,
				DarkMode: themeModel.DarkMode,
			},
			IconHash: iconhash,
		}
	}

	return users, nil

}
