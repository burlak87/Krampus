package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"time"

	"crampus/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type UserStorage interface {
	InsertUser(user domain.User) (int64, error)
	SelectUsers(email string) (domain.User, error)
	SelectUserByID(userID int64) (domain.User, error)
	RefreshStore(userID int64, token string, expiresAt time.Time) error
	RefreshGet(token string) (int64, error)
	RefreshDelete(token string) error
	RefreshDeleteByUserID(userID int64) error
	UserBlocked(email string, windowStart time.Time) ([]map[string]interface{}, error)
	LogAttempt(email string, result bool, attemptTime time.Time) error
	GetFailedLogAttempts(email string, windowStart time.Time) (int, error)
	BlockUser(email, blockedUntil string) error
	RenovationTwoFAStatus(userID int64, enabled bool) error
	InsertTwoFaCode(userID int64, code string, expiresAt time.Time) error
	SelectTwoFaCodeByUserID(userID int64) (domain.TwoFaCode, error)
	RenovationTwoFaCodeAttempts(codeID int64, attempts int) error
	MarkTwoFaCodeUsed(codeID int64) error
	SelectRecentCodeRequests(userID int64, since time.Time) (int, error)
	SelectRecentVerificationAttempts(userID int64, since time.Time) (int, error)
}

type User struct {
	storage      UserStorage
	jwtSecret    string
}

func NewUser(storage UserStorage, jwt string) *User{
	return &User{storage: storage, jwtSecret: jwt}
}

func (s *User) UserRegister(user domain.User) (domain.User, error) {
	fmt.Printf("DEBUG SERVICE REGISTER: Starting registration for: %s\n", user.Email)
	
	if user.Username == "" || user.Firstname == "" || user.Lastname == "" || user.Email == "" {
		return domain.User{}, errors.New("Invalid input: all fields are required")
	}
	
	if user.Password == "" || len(user.Password) < 8 {
		return domain.User{}, errors.New("Invalid password input: password must br at least 8 characters")
	}
	
	hasLetters, _ := regexp.MatchString(`[a-zA-Zа-яА-Я]`, user.Password)
	hasDigits, _ := regexp.MatchString(`[0-9]`, user.Password)
	hasSpecial, _ := regexp.MatchString(`[^a-zA-Zа-яА-Я0-9\s]`, user.Password)
	
	if !hasLetters || !hasDigits || !hasSpecial {
		return domain.User{}, errors.New("Invalid password input: password must contain letters, digits and special characters")
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, errors.New("Error hashing password")
	}
	
	userToSave := domain.User{
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: string(hash),
		TwoFAEnabled: user.TwoFAEnabled,
	}
	
	fmt.Printf("DEBUG SERVICE REGISTER: Calling storage.InsertUser\n")
	id, err := s.storage.InsertUser(userToSave)
	if err != nil {
		fmt.Printf("DEBUG SERVICE REGISTER: Storage error: %v\n", err)
		return domain.User{}, err
	}
	
	createdUser := domain.User{
		ID: id,
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFAEnabled,
		CreatedAt:    time.Now(),
	}
	
	fmt.Printf("DEBUG SERVICE REGISTER: SUCCESS - Created student with ID: %d\n", id)
	return createdUser, nil
}

func (s *User) UserLogin(user domain.User) (domain.TokenResponse, domain.TwoFaCodes, error) {
    fmt.Printf("DEBUG LOGIN: Attempting login for email: '%s'\n", user.Email)
    fmt.Printf("DEBUG LOGIN: Password provided: '%s'\n", user.Password)
    fmt.Printf("DEBUG LOGIN: TwoFA enabled: '%v'\n", user.TwoFAEnabled)
    
    if user.Email == "" || user.Password == "" {
        fmt.Printf("DEBUG LOGIN: Email or password empty\n")
        return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("email and password are required")
    }
    
    blocked, minutesLeft, err := s.IsUserBlocked(user.Email)
    if err != nil {
        fmt.Printf("DEBUG LOGIN: Error checking block status: %v\n", err)
        return domain.TokenResponse{}, domain.TwoFaCodes{}, err
    }
    
    if blocked {
        fmt.Printf("DEBUG LOGIN: User is blocked for %d minutes\n", minutesLeft)
        return domain.TokenResponse{}, domain.TwoFaCodes{}, fmt.Errorf("your account is blocked for %d minutes", minutesLeft)
    }
    
    fmt.Printf("DEBUG LOGIN: Searching user in database...\n")
    dbUser, err := s.storage.SelectUsers(user.Email)
    if err != nil {
        fmt.Printf("DEBUG LOGIN: Database error or user not found: %v\n", err)
        s.LogLoginAttempt(user.Email, false)
        return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("invalid credentials")
    }
    
    fmt.Printf("DEBUG LOGIN: User found - ID: %d, Email: %s\n", dbUser.ID, dbUser.Email)
    fmt.Printf("DEBUG LOGIN: Stored password hash: %s\n", dbUser.PasswordHash)
    fmt.Printf("DEBUG LOGIN: Provided password: %s\n", user.Password)
    
    fmt.Printf("DEBUG LOGIN: Comparing passwords...\n")
    err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(user.Password))
    if err != nil {
        fmt.Printf("DEBUG LOGIN: Password comparison failed: %v\n", err)
        s.LogLoginAttempt(user.Email, false)
        return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("invalid credentials")
    }
    
    fmt.Printf("DEBUG LOGIN: Password correct!\n")
    
    attempts, err := s.GetFailedAttempts(user.Email)
    if err != nil {
        fmt.Printf("DEBUG LOGIN: Error getting failed attempts: %v\n", err)
        return domain.TokenResponse{}, domain.TwoFaCodes{}, err
    }
    
    maxAttempts := int64(5)
    if attempts >= maxAttempts {
        fmt.Printf("DEBUG LOGIN: Too many failed attempts: %d\n", attempts)
        s.BlockUser(user.Email)
        return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("too many failed attempts, account blocked")
    }

    // if dbUser.TwoFAEnabled {
    if dbUser.TwoFAEnabled != false {
        tempToken, err := s.GenerateTempToken(dbUser.ID)
        if err != nil {
            fmt.Printf("DEBUG LOGIN: Error generating temp token: %v\n", err)
            return domain.TokenResponse{}, domain.TwoFaCodes{}, err
        }
        return domain.TokenResponse{}, domain.TwoFaCodes{RequiresTwoFa: true, TempToken: tempToken}, nil
    }
    
    accessToken, err := s.GenerateAccessToken(dbUser.ID)
    if err != nil {
        fmt.Printf("DEBUG LOGIN: Error generating access token: %v\n", err)
        return domain.TokenResponse{}, domain.TwoFaCodes{}, err
    }
    
    refreshToken, err := s.GenerateRefreshToken(dbUser.ID)
    if err != nil {
        fmt.Printf("DEBUG LOGIN: Error generating refresh token: %v\n", err)
        return domain.TokenResponse{}, domain.TwoFaCodes{}, err
    }
    
    s.LogLoginAttempt(user.Email, true)
    fmt.Printf("DEBUG LOGIN: Login successful for user ID: %d\n", dbUser.ID)
    return domain.TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken}, domain.TwoFaCodes{}, nil
}

func (s * User) UserLogout(userID int64, password string) error {
    fmt.Printf("DEBUG LOGOUT: Searching user in database...\n")
    dbUser, err := s.storage.SelectUserByID(userID)
    if err != nil {
        fmt.Printf("DEBUG LOGOUT: Database error or user not found: %v\n", err)
        s.LogLoginAttempt(dbUser.Email, false)
        return errors.New("invalid credentials")
    }
    
    fmt.Printf("DEBUG LOGOUT: User found - ID: %d, Email: %s\n", dbUser.ID, dbUser.Email)
    fmt.Printf("DEBUG LOGOUT: Stored password hash: %s\n", dbUser.PasswordHash)
    fmt.Printf("DEBUG LOGOUT: Provided password: %s\n", dbUser.Password)
    
    fmt.Printf("DEBUG LOGOUT: Comparing passwords...\n")
    err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(dbUser.Password))
    if err != nil {
        fmt.Printf("DEBUG LOGOUT: Password comparison failed: %v\n", err)
        s.LogLoginAttempt(dbUser.Email, false)
        return errors.New("invalid credentials")
    }
    
    fmt.Printf("DEBUG LOGOUT: Password correct!\n")

    err = s.storage.RefreshDeleteByUserID(userID)
    if err != nil {
    	return err
    }
    
    fmt.Printf("DEBUG LOGOUT: User %d successful logged out\n", userID)
	return nil
}

func (s *User) UserRefresh(refreshToken string) (domain.TokenResponse, error) {
	userID, err := s.storage.RefreshGet(refreshToken)
	if err != nil {
		return  domain.TokenResponse{}, errors.New("Invalid refresh token")
	}

	accessToken, err := s.GenerateAccessToken(userID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	newRefreshToken, err := s.GenerateRefreshToken(userID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	s.storage.RefreshDelete(refreshToken)
	
	return domain.TokenResponse{AccessToken: accessToken, RefreshToken: newRefreshToken}, nil
}

func (s *User) GenerateAccessToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *User) GenerateTempToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(10 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *User) GenerateRefreshToken(id int64) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	
	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = id
	claims["exp"] = time.Now().Add(7 *24 * time.Hour).Unix()
	
	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", err
	}
	
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	err = s.storage.RefreshStore(id, signed, expiresAt)
	return signed, err
}

func (s *User) IsUserBlocked(email string) (bool, int64, error) {
	now := time.Now().UTC()
	windowStart := now
	
	result, err := s.storage.UserBlocked(email, windowStart)
	if err != nil {
		fmt.Printf("Ошибка проверки блокировки: %v\n", err)
		return false, 0, err
	}
	
	if len(result) > 0 {
		blockedUntilStr, ok := result[0]["blocked_until"].(string)
		if !ok {
			return false, 0, errors.New("invalid format for blocked_until")
		}
	
		blockedUntil, err := time.Parse(time.RFC3339, blockedUntilStr)
		if err != nil {
			return false, 0, err
		}
	
		minutesLeft := math.Ceil(time.Until(blockedUntil).Minutes())
		if minutesLeft < 0 {
			minutesLeft = 0
		}
	
		return true, int64(minutesLeft), nil
	}
	
	return false, 0, nil
}

func (s *User) LogLoginAttempt(email string, result bool) {
	attemptTime := time.Now().UTC()

	err := s.storage.LogAttempt(email, result, attemptTime)
	if err != nil {
		fmt.Printf("Ошибка логирования: %v\n", err)
	}
}

func (s *User) GetFailedAttempts(email string) (int64, error) {
	now := time.Now().UTC()
	windowStart := now.Add(-1 * time.Minute)
	
	count, err := s.storage.GetFailedLogAttempts(email, windowStart)
	if err != nil {
		fmt.Printf("Ошибка подсчета попыток: %v\n", err)
		return int64(0), err
	}

	return int64(count), err
}

func (s *User) BlockUser(email string) {
	now := time.Now()
	blockedUntil := now.Add(1 * time.Minute).Format(time.RFC3339)

	s.LogLoginAttempt(email, false)

	err := s.storage.BlockUser(email, blockedUntil)
	if err != nil {
		fmt.Printf("Ошибка блокировки: %v\n", err)
	}
}

func (s *User) UsersSendEmailCode(tempToken string) error {
	userID, err := s.extractUserIDFromToken(tempToken)
	if err != nil {
		return errors.New("Invalid temp token")
	}
	
	fifteenMinutesAgo := time.Now().Add(-15 * time.Minute)
	recentRequests, err := s.storage.SelectRecentCodeRequests(userID, fifteenMinutesAgo)
	if err != nil {
		return err
	}
	
	if recentRequests >= 3 {
		return errors.New("too many code requests, please try again later")
	}
	
	code, err := s.generateSixDigitCode()
	if err != nil {
		return errors.New("failed to generate code")
	}
	
	expiresAt := time.Now().Add(5 * time.Minute)
	err = s.storage.InsertTwoFaCode(userID, code, expiresAt)
	if err != nil {
		return err
	}
	
	err = s.sendEmail(userID, code)
	if err != nil {
		return err
	}
	
	return nil
}

func (s *User) VerifyCode(code domain.Code) (domain.TokenResponse, error) {
	userID, err := s.extractUserIDFromToken(code.TempToken)
	if err != nil {
		return domain.TokenResponse{}, errors.New("invalid temp token")
	}
	
	tenMinuteAgo := time.Now().Add(-10 * time.Minute)
	recentAttempts, err := s.storage.SelectRecentVerificationAttempts(userID, tenMinuteAgo)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	
	if recentAttempts >= 5 {
		return domain.TokenResponse{}, errors.New("too many verification attempts, please try again later")
	}
	
	twoFaCode, err := s.storage.SelectTwoFaCodeByUserID(userID)
	if err != nil {
		return domain.TokenResponse{}, errors.New("invalid temp token or code not found")
	}
	
	if twoFaCode.IsUsed {
		return domain.TokenResponse{}, errors.New("code already used")
	}
	
	if twoFaCode.Attempts >= 3 {
		return domain.TokenResponse{}, errors.New("too many attempts")
	}
	
	if time.Now().After(twoFaCode.ExpiresAt) {
		return domain.TokenResponse{}, errors.New("code expires")
	}
	
	if twoFaCode.Code != code.Code {
		err = s.storage.RenovationTwoFaCodeAttempts(twoFaCode.ID, twoFaCode.Attempts+1)
		if err != nil {
			return domain.TokenResponse{}, err
		}
		
		remainingAttempts := 3 - (twoFaCode.Attempts + 1)
		return domain.TokenResponse{}, fmt.Errorf("invalid code, %d attempts remaining", remainingAttempts)
	}
	
	err = s.storage.MarkTwoFaCodeUsed(twoFaCode.ID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	
	accessToken, err := s.GenerateAccessToken(twoFaCode.UserID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	
	refreshToken, err := s.GenerateRefreshToken(twoFaCode.UserID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	
	return domain.TokenResponse{
		AccessToken: accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *User) generateSixDigitCode() (string, error) {
	max := big.NewInt(899999)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", nil
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

func (s *User) sendEmail(userID int64, code string) error {
	fmt.Printf("Sending email to user %d: Your code: %s (valid for 5 minutes)\n", userID, code)
	return nil
}

func (s *User) EnableTwoFA(userID int64) error {
	return s.storage.RenovationTwoFAStatus(userID, true)
}

func (s *User) DisableTwoFA(userID int64, password string) error {
	student, err := s.storage.SelectUserByID(userID)
	if err != nil {
		return errors.New("user not found")
	}
	
	err = bcrypt.CompareHashAndPassword([]byte(student.PasswordHash), []byte(password))
	if err != nil {
		return errors.New("Invalid password")
	}
	
	return s.storage.RenovationTwoFAStatus(userID, false)
}

func (s *User) extractUserIDFromToken(tokenString string) (int64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return 0, err
	}
	
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("Invalid token claims")
	}
	
	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, errors.New("Invalid user_id in token")
	}
	
	return int64(userIDFloat), nil
}

func (s *User) UserSendEmailCode(tempToken string) error {
    // Обертка над UserSendEmailCode
    return s.UserSendEmailCode(tempToken)
}

// func (s *User) startTokenCleanupService(interval time.Duration) {
// 	ticker := time.NewTicker(internal)
// 	defer ticker.Stop()
	
// 	for range ticker.C {
// 		ctx := context.Background()
// 		errRefresh := s.storage.DeleteExpiredRefreshTokens(ctx)
// 		if errRefresh != nil {
// 			log.Printf("Error cleaning expired tokens: %v", errRefresh)
// 		}
		
// 		errAccess := s.storage.DeleteExpiredAccessTokens(ctx)
// 		if errAccess != nil {
// 			log.Printf("Error cleaning expired tokens: %v", errAccess)
// 		}
		
// 		errTemp := s.storage.DeleteExpiredTempTokens(ctx)
// 		if errTemp != nil {
// 			log.Printf("Error cleaning expired tokens: %v", errTemp)
// 		}
		
// 		errTwoFaCodes := s.storage.DeleteExpiredTwoFaCodes(ctx)
// 		if errTwoFaCodes != nil {
// 			log.Printf("Error cleaning expired codes: %v", errTwoFaCodes)
// 		}
// 	}
// }