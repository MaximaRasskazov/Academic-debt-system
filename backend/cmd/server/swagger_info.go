// Package main Academic Debt System API
//
//	@title		Academic Debt System API
//	@version	1.0
//	@description	REST API системы учёта академических задолженностей.
//	@description	Аутентификация — Bearer JWT. Получить токен: POST /api/auth/login.
//
//	@host		localhost:8080
//	@BasePath	/
//	@schemes	http
//
//	@securityDefinitions.apikey	BearerAuth
//	@in				header
//	@name				Authorization
//	@description		Формат: "Bearer <access_token>"
package main