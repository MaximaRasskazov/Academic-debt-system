package auth

import "golang.org/x/crypto/bcrypt"

// bcryptCost — параметр сложности bcrypt. 10 — рекомендованный
// производственный default, ~70ms на современном CPU. Выше — больше
// нагрузка на сервер, ниже — менее устойчиво к перебору.
const bcryptCost = 10

func hashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func checkPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

// dummyHash — заранее подготовленный валидный bcrypt-хеш, используется
// в Login против несуществующего email, чтобы суммарное время ответа
// не отличалось от ответа с проверкой реального пароля. Защита от
// user enumeration по тайминговому каналу.
//
// Получен как bcrypt.GenerateFromPassword([]byte("never-matches"), 10).
var dummyHash = "$2a$10$rIYOPbqLddv7tXqQrUOAfeRWh6.gQK5xRwlpVUKDx5IcGzNgYIBl."
