#include "RestTestCtrl.h"

// Add definition of your processing function here

namespace api
{
    namespace v1
    {
        void User::getInfo(const HttpRequestPtr &req, std::function<void(const HttpResponsePtr &)> &&callback, int userId) const
        {
            auto resp = HttpResponse::newHttpResponse();
            resp->setBody("Here is a User Info\n");
            resp->setExpiredTime(0);
            callback(resp);
        }
        void User::getDetailInfo(const HttpRequestPtr &req, std::function<void(const HttpResponsePtr &)> &&callback, int userId) const
        {
            auto resp = HttpResponse::newHttpResponse();
            resp->setBody("Here is a User Detail Info\n");
            resp->setExpiredTime(0);
            callback(resp);
        }
        void User::newUser(const HttpRequestPtr &req, std::function<void(const HttpResponsePtr &)> &&callback, std::string &&userName)
        {
            auto resp = HttpResponse::newHttpResponse();
            resp->setBody("New User added\n");
            resp->setExpiredTime(0);
            callback(resp);
        }
    } // namespace v1
} // namespace api