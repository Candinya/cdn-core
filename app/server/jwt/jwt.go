package jwt

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
)

type JWT struct {
	key []byte
}

type User struct {
	ID      uint
	Expires int64 // Unix second
}

func New(key string) (*JWT, error) {
	if len(key) == 0 {
		return nil, errors.New("key is empty")
	}

	return &JWT{key: []byte(key)}, nil
}

func (j *JWT) ParseUser(tokenString string) (*User, error) {
	// 检查是否有效
	if len(tokenString) == 0 {
		return nil, errors.New("token string is empty")
	}

	// 映射字段
	user := &User{}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return j.key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse jwt failed: %w", err)
	}

	// 匹配内容
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		user.ID = uint(claims["id"].(float64))
		user.Expires = int64(claims["exp"].(float64))
	} else {
		return nil, fmt.Errorf("invalid token")
	}

	return user, nil
}

func (j *JWT) SignToken(user *User) (string, error) {
	// 创建声明
	claims := jwt.MapClaims{
		"id":  user.ID,
		"exp": user.Expires,
	}

	// 创建令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名并返回
	return token.SignedString(j.key)
}
