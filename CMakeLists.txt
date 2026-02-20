cmake_minimum_required(VERSION 3.10)
project(crm_be VERSION 0.1)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

set_property(DIRECTORY APPEND PROPERTY CMAKE_CONFIGURE_DEPENDS "${CMAKE_CURRENT_SOURCE_DIR}/config/config.json")

find_package(PostgreSQL REQUIRED)

include(FetchContent)

set(BUILD_SHARED_LIBS OFF CACHE BOOL "" FORCE)
set(BUILD_OBJECT_LIBS OFF CACHE BOOL "" FORCE)

FetchContent_Declare(jsoncpp
	GIT_REPOSITORY 	https://github.com/open-source-parsers/jsoncpp.git
	GIT_TAG 1.9.5
	SOURCE_DIR 	${CMAKE_CURRENT_SOURCE_DIR}/external/jsoncpp
)
set(JSONCPP_WITH_TESTS OFF CACHE BOOL "" FORCE)
set(JSONCPP_WITH_POST_BUILD_UNITTEST OFF CACHE BOOL "" FORCE)
FetchContent_MakeAvailable(jsoncpp)

FetchContent_Declare(drogon
	GIT_REPOSITORY 	https://github.com/drogonframework/drogon
	GIT_TAG v1.9.12
	SOURCE_DIR 	${CMAKE_CURRENT_SOURCE_DIR}/external/drogon
)
set(JSONCPP_LIBRARIES ${CMAKE_CURRENT_BINARY_DIR}/lib/libjsoncpp.a CACHE STRING "" FORCE)
set(JSONCPP_INCLUDE_DIRS ${CMAKE_CURRENT_SOURCE_DIR}/external/jsoncpp/include CACHE STRING "" FORCE)
set(BUILD_POSTGRESQ ON CACHE BOOL "" FORCE)
set(BUILD_MYSQL OFF CACHE BOOL "" FORCE)
set(BUILD_REDIS OFF CACHE BOOL "" FORCE) # Позже может потребоваться включить Redis, должно ускорить работу программы
set(BUILD_SQLITE OFF CACHE BOOL "" FORCE)
set(BUILD_TESTING OFF CACHE BOOL "" FORCE)
set(BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(BUILD_DOC OFF CACHE BOOL "" FORCE)
set(USE_STATIC_LIBS_ONLY ON CACHE BOOL "" FORCE)
FetchContent_MakeAvailable(drogon)

set(DROGON_GEN_HANDLERS_DIR 
	${CMAKE_CURRENT_SOURCE_DIR}/src/drogon_gen/handlers)
set(DROGON_GEN_MODELS_DIR 
	${CMAKE_CURRENT_SOURCE_DIR}/src/drogon_gen/models)
set(DROGON_GEN_HANDLERS_SOURCES 
	${DROGON_GEN_HANDLERS_DIR}/Test/TestCtrl.cc
	${DROGON_GEN_HANDLERS_DIR}/JsonTest/JsonTestCtrl.cc
	${DROGON_GEN_HANDLERS_DIR}/RestTest/RestTestCtrl.cc)
set(DROGON_GEN_MODELS_SOURCES "")

add_executable(${PROJECT_NAME} ${CMAKE_CURRENT_SOURCE_DIR}/src/main.cpp ${DROGON_GEN_HANDLERS_SOURCES}) # Сюда модели добавить, когда будут

set_target_properties(${PROJECT_NAME} PROPERTIES
    RUNTIME_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/bin"
)

file(COPY ${CMAKE_CURRENT_SOURCE_DIR}/config/config.json
    DESTINATION ${CMAKE_BINARY_DIR}/config)

target_link_libraries(${PROJECT_NAME} PRIVATE 
	drogon
	${PostgreSQL_LIBRARIES}
    ldap
    gssapi_krb5
    ssl
    crypto
)
target_include_directories(${PROJECT_NAME} PRIVATE ${DROGON_INCLUDE_DIR} ${DROGON_GEN_HANDLERS_DIR}/* ${DROGON_GEN_MODELS_DIR}/*)
target_compile_options(${PROJECT_NAME} PRIVATE "-Werror" "-Wall" "-Wpedantic" -O3) 