package service

// func (s *User) generateSixDigitCode() (string, error) {
// 	max := big.NewInt(899999)
// 	n, err := rand.Int(rand.Reader, max)
// 	if err != nil {
// 		return "", nil
// 	}
// 	return fmt.Sprintf("%06d", n.Int64()+100000), nil
// }

// func (s *User) sendEmail(userID int64, code string) error {
// 	fmt.Printf("Sending email to user %d: Your code: %s (valid for 5 minutes)\n", userID, code)
// 	return nil
// }

// func (s *User) DisableTwoFA(userID int64, password string) error {
// 	student, err := s.userStorage.SelectUserByID(userID)
// 	if err != nil {
// 		return errors.New("user not found")
// 	}

// 	err = bcrypt.CompareHashAndPassword([]byte(student.PasswordHash), []byte(password))
// 	if err != nil {
// 		return errors.New("Invalid password")
// 	}

// 	return s.twoFAStorage.RenovationTwoFAStatus(userID, false)
// }

// func (s *User) extractUserIDFromToken(tokenString string) (int64, error) {
// 	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
// 		return []byte(s.jwtSecret), nil
// 	})
// 	if err != nil {
// 		return 0, err
// 	}

// 	claims, ok := token.Claims.(jwt.MapClaims)
// 	if !ok {
// 		return 0, errors.New("Invalid token claims")
// 	}

// 	userIDFloat, ok := claims["user_id"].(float64)
// 	if !ok {
// 		return 0, errors.New("Invalid user_id in token")
// 	}

// 	return int64(userIDFloat), nil
// }

// func (s *User) UserSendEmailCode(tempToken string) error {
// 	// Обертка над UserSendEmailCode
// 	return s.UserSendEmailCode(tempToken)
// }

// // func (s *User) startTokenCleanupService(interval time.Duration) {
// // 	ticker := time.NewTicker(internal)
// // 	defer ticker.Stop()

// // 	for range ticker.C {
// // 		ctx := context.Background()
// // 		errRefresh := s.storage.DeleteExpiredRefreshTokens(ctx)
// // 		if errRefresh != nil {
// // 			log.Printf("Error cleaning expired tokens: %v", errRefresh)
// // 		}

// // 		errAccess := s.storage.DeleteExpiredAccessTokens(ctx)
// // 		if errAccess != nil {
// // 			log.Printf("Error cleaning expired tokens: %v", errAccess)
// // 		}

// // 		errTemp := s.storage.DeleteExpiredTempTokens(ctx)
// // 		if errTemp != nil {
// // 			log.Printf("Error cleaning expired tokens: %v", errTemp)
// // 		}

// // 		errTwoFaCodes := s.storage.DeleteExpiredTwoFaCodes(ctx)
// // 		if errTwoFaCodes != nil {
// // 			log.Printf("Error cleaning expired codes: %v", errTwoFaCodes)
// // 		}
// // 	}
// // }
