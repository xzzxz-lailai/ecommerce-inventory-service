package pkg

import (
	"errors"
	"time"

	"inventory-service/config"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	SubjectType   string `json:"subject_type"`
	UserID        int64  `json:"user_id,omitempty"`
	RoleID        int8   `json:"role_id,omitempty"`
	PartnerID     int64  `json:"partner_id,omitempty"`
	PartnerUserID int64  `json:"partner_user_id,omitempty"`
	jwt.RegisteredClaims
}

// GenerateToken 生成公司内部账号 JWT Token。
func GenerateToken(userID int64, roleID int8) (string, error) {
	claims := Claims{
		SubjectType: "user",
		UserID:      userID,
		RoleID:      roleID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.Cfg.JWT.Expire) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.Cfg.JWT.Secret))
}

// ParseToken 解析由 user-service 签发的 JWT Token。
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("token 签名算法错误")
			}
			return []byte(config.Cfg.JWT.Secret), nil
		},
	)
	if err != nil || !token.Valid {
		return nil, errors.New("token 无效或已过期")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("token 解析失败")
	}

	return claims, nil
}
