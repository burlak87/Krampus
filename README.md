role UserRole @default(REGULAR)
displayName string
picture string

isVerified Boolean @default(false) @map("is_verified")
idTwoFactorEnabled Boolean @default(false) @map("is_two_factor_enabled")

method AuthMethod

accounts Account[]

enum UserRole {
  REGULAR
  ADMIN
}

enum AuthMethod {
  CREDENTIALS
  GOOGLE
  YANDEX  
}

enum TokenType {
  VERIFICATION
  TWO_FACTOR
  PASSWORD_RESET
}

Account

type string
provider string
refreshToken string?
accessToken string?
expiresAt Int
user User?
userId string?

Token

email string
token string
type TokenType
expiresIn DateTime 


!
Нужно написать метод выхода из аккаунта. В котором должна удаляться сессия из Redis и отчистка сессии.  
Нужно добавить поиск по имени пользователя(@username), по емейлу (burlak87@gmail.com), чтобы они были уникальны.
Добавить отправку кода на почту.
Добавить проверку почты. 
Добавить Passkey.
Написать сессии, которые будут вызываться в большинстве методов. Они должны сохраняться в Redis. В сессии будет храниться айди, по которому мы после будет совершать поиск.
Разработать поиск по сессии и по бд в сущности пользователь
Создать декоратор для ролей пользователей. с ключами ролей. Так же создадим guard для ролей. 
Добавить middleware для проверок.
Создать декоратор или миддлевару для проверки ролей в нужных местах.
Добавить Profile. 
