cmake_minimum_required(VERSION 3.10)
project(crm_be VERSION 0.1)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(jsoncpp REQUIRED)

if(NOT jsoncpp_FOUND)
	message(FATAL_ERROR "jsoncpp not found!")
endif()

include(FetchContent)

FetchContent_Declare(drogon
	GIT_REPOSITORY 	https://github.com/drogonframework/drogon
	SOURCE_DIR 	${CMAKE_CURRENT_SOURCE_DIR}/external/drogon
)
set(JSONCPP_LIBRARIES jsoncpp CACHE STRING "" FORCE)
set(BUILD_POSTGRESQ ON CACHE BOOL "" FORCE)
set(BUILD_TESTING OFF CACHE BOOL "" FORCE)
set(BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(BUILD_DOC OFF CACHE BOOL "" FORCE)
set(BUILD_SHARED_LIBS OFF CACHE BOOL "" FORCE)
set(USE_STATIC_LIBS_ONLY ON CACHE BOOL "" FORCE)
FetchContent_MakeAvailable(drogon)

set(DROGON_GEN_DIR 
	${CMAKE_CURRENT_SOURCE_DIR}/src/drogon_gen)
set(DROGON_GEN_SOURCES 
	${DROGON_GEN_DIR}/Test/TestCtrl.cc
	${DROGON_GEN_DIR}/JsonTest/JsonTestCtrl.cc
	${DROGON_GEN_DIR}/RestTest/RestTestCtrl.cc)

add_executable(${PROJECT_NAME} ${CMAKE_CURRENT_SOURCE_DIR}/src/main.cpp ${DROGON_GEN_SOURCES})

set_target_properties(${PROJECT_NAME} PROPERTIES
    RUNTIME_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/bin"
)

add_custom_command(TARGET ${PROJECT_NAME} POST_BUILD
    COMMAND ${CMAKE_COMMAND} -E copy_directory
    ${CMAKE_CURRENT_SOURCE_DIR}/config
    $<TARGET_FILE_DIR:${PROJECT_NAME}>/../config
)

target_link_libraries(${PROJECT_NAME} PRIVATE drogon)
target_include_directories(${PROJECT_NAME} PRIVATE ${DROGON_INCLUDE_DIR} ${DROGON_GEN_DIR}/*)
target_compile_options(${PROJECT_NAME} PRIVATE "-Werror" "-Wall" "-Wpedantic" -O3) 