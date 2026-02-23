#include "Auth.h"
#include <drogon/HttpResponse.h>
#include <drogon/HttpRequest.h>
#include <drogon/utils/Utilities.h>
#include <json/json.h>
#include <string>
#include <openssl/evp.h>
#include <openssl/rand.h>
#include <crypt.h>
#include <iomanip>
#include <sstream>

using namespace api::v1;

std::string generateRandomToken(size_t byteLength = 32)
{
    std::vector<unsigned char> buffer(byteLength);
    if (RAND_bytes(buffer.data(), byteLength) != 1)
    {
        throw std::runtime_error("Failed to generate random bytes");
    }
    std::ostringstream oss;
    oss << std::hex << std::setfill('0');
    for (auto b : buffer)
    {
        oss << std::setw(2) << static_cast<int>(b);
    }
    return oss.str();
}

std::string sha256(const std::string &input)
{
    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    const EVP_MD *md = EVP_sha256();
    unsigned char hash[EVP_MAX_MD_SIZE];
    unsigned int hash_length;

    EVP_DigestInit_ex(ctx, md, nullptr);
    EVP_DigestUpdate(ctx, input.data(), input.size());
    EVP_DigestFinal_ex(ctx, hash, &hash_length);
    EVP_MD_CTX_free(ctx);

    std::ostringstream oss;
    oss << std::hex << std::setfill('0');
    for (unsigned int i = 0; i < hash_length; ++i)
    {
        oss << std::setw(2) << static_cast<int>(hash[i]);
    }
    return oss.str();
}

bool checkPassword(const std::string &password, const std::string &password_hash)
{
    struct crypt_data data;
    data.initialized = 0;
    char *result = crypt_r(password.c_str(), password_hash.c_str(), &data);
    return result != nullptr && password_hash == result;
}

// curl -v -H "Authorization: Basic dXNlcjpwYXNz" -X POST http://localhost:8080/api/v1/auth/login
void Auth::login(const HttpRequestPtr &req, std::function<void(const HttpResponsePtr &)> &&callback)
{
    auto authHeader = req->getHeader("Authorization");
    if (authHeader.empty() || authHeader.substr(0, 6) != "Basic ")
    {
        Json::Value ret;
        ret["error"] = "Missing or invalid Authorization header. Expected Basic auth.";
        auto resp = HttpResponse::newHttpJsonResponse(ret);
        resp->setStatusCode(k401Unauthorized);
        resp->addHeader("WWW-Authenticate", "Basic realm=\"Login\"");
        callback(resp);
        return;
    }

    std::string base64Credentials = authHeader.substr(6);
    std::string decoded = drogon::utils::base64Decode(base64Credentials);
    size_t colonPos = decoded.find(':');
    if (colonPos == std::string::npos)
    {
        Json::Value ret;
        ret["error"] = "Invalid Basic auth format";
        auto resp = HttpResponse::newHttpJsonResponse(ret);
        resp->setStatusCode(k400BadRequest);
        callback(resp);
        return;
    }

    std::string login = decoded.substr(0, colonPos);
    std::string password = decoded.substr(colonPos + 1);

    __attribute_maybe_unused__ int account_id = 1;                                              // ToDo: Достать из БД через соответствие с логином
    std::string password_hash = "$2b$12$xuq9ombFmHy3IZrRHTxcDem1Y3JtAZ6bWQwrXmtLfJtFZmilt/Jta"; // ToDo: Достать из БД по account_id. В примере пароль: pass

    if (!checkPassword(password, password_hash))
    {
        Json::Value ret;
        ret["error"] = "Invalid username or password";
        auto resp = HttpResponse::newHttpJsonResponse(ret);
        resp->setStatusCode(k401Unauthorized);
        callback(resp);
        return;
    }

    // ToDo: Добавить проверку на наличие дубликатов в БД
    std::string access_token = generateRandomToken(32);
    std::string refresh_token = generateRandomToken(32);
    std::string access_token_hash = sha256(access_token);
    std::string refresh_token_hash = sha256(refresh_token);
    // ToDo: Загрузить в БД токены. Добавить запись о новой сессии

    Json::Value ret;
    ret["access_token"] = access_token;
    ret["refresh_token"] = refresh_token;
    ret["token_type"] = "Bearer";

    auto resp = HttpResponse::newHttpJsonResponse(ret);
    resp->setStatusCode(k200OK);
    callback(resp);
}

// curl -v -H "Content-Type: application/json" -d '{"refresh_token": "refresh_token"}' -X POST http://localhost:8080/api/v1/auth/refresh
void Auth::refresh(const HttpRequestPtr &req, std::function<void(const HttpResponsePtr &)> &&callback)
{
    auto json = req->getJsonObject();
    if (!json || !json->isMember("refresh_token"))
    {
        Json::Value ret;
        ret["error"] = "Missing refresh_token in body";
        auto resp = HttpResponse::newHttpJsonResponse(ret);
        resp->setStatusCode(k400BadRequest);
        callback(resp);
        return;
    }

    std::string old_refresh_token = (*json)["refresh_token"].asString();
    std::string old_refresh_token_hash = sha256(old_refresh_token);

    // ToDo: Найти сессию по refresh_hash и проверить срок жизни токена. account_id достать из соответствия с токеном
    bool is_valid = (old_refresh_token.find("refresh_token") != std::string::npos); // Не так, это заглушка

    if (!is_valid)
    {
        Json::Value ret;
        ret["error"] = "Invalid or expired refresh token";
        auto resp = HttpResponse::newHttpJsonResponse(ret);
        resp->setStatusCode(k401Unauthorized);
        callback(resp);
        return;
    }

    // ToDo: Если все окей, как вариант можно удалить старую сессию и создать новую вместе с новой парой ключей

    // ToDo: Добавить проверку на наличие дубликатов в БД
    std::string new_access_token = generateRandomToken(32);
    std::string new_refresh_token = generateRandomToken(32);
    std::string new_access_token_hash = sha256(new_access_token);
    std::string new_refresh_token_hash = sha256(new_refresh_token);
    // ToDo: Загрузить в БД токены. Добавить запись о новой сессии

    Json::Value ret;
    ret["access_token"] = new_access_token;
    ret["refresh_token"] = new_refresh_token;
    ret["token_type"] = "Bearer";

    auto resp = HttpResponse::newHttpJsonResponse(ret);
    resp->setStatusCode(k200OK);
    callback(resp);
}

// curl -v -H "Authorization: Bearer a40a67029068bc23fc79addbf6c9505df7f9c1b460269c06736293fae154d548" -X POST http://localhost:8080/api/v1/auth/logout
void Auth::logout(const HttpRequestPtr &req, std::function<void(const HttpResponsePtr &)> &&callback)
{
    auto authHeader = req->getHeader("Authorization");
    if (authHeader.empty() || authHeader.substr(0, 7) != "Bearer ")
    {
        Json::Value ret;
        ret["error"] = "Missing or invalid Authorization header. Expected Bearer token.";
        auto resp = HttpResponse::newHttpJsonResponse(ret);
        resp->setStatusCode(k401Unauthorized);
        callback(resp);
        return;
    }

    std::string access_token = authHeader.substr(7);
    std::string access_token_hash = sha256(access_token);

    // ToDo: Удалить сессию по access_token_hash

    auto resp = HttpResponse::newHttpResponse();
    resp->setStatusCode(k204NoContent);
    callback(resp);
}