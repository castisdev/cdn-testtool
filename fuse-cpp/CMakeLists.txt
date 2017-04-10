cmake_minimum_required(VERSION 2.8)
set(PROJECT_NAME fuse-fs)
project(${PROJECT_NAME})
set(Boost_USE_STATIC_LIBS ON)
find_package(Boost 1.56.0 COMPONENTS filesystem system REQUIRED)
add_definitions(-Wall -D_REENTRANT -D_GNU_SOURCE -D_UNICODE -g -Os)

include(FindPkgConfig)
pkg_check_modules(FUSE REQUIRED fuse)
include_directories(
    SYSTEM ${Boost_INCLUDE_DIRS}
    SYSTEM ${FUSE_INCLUDE_DIRS}
)
link_directories(${FUSE_LIBRARY_DIRS})

add_executable(fuse-cpp-directio fusefs_directio.cpp)
set_target_properties(fuse-cpp-directio PROPERTIES COMPILE_FLAGS "-std=c++1y ${FUSE_CFLAGS_OTHER}")
target_link_libraries(fuse-cpp-directio ${FUSE_LIBRARIES} ${Boost_LIBRARIES})

add_executable(fuse-cpp-bufferedio fusefs_bufferedio.cpp)
set_target_properties(fuse-cpp-bufferedio PROPERTIES COMPILE_FLAGS "-std=c++1y ${FUSE_CFLAGS_OTHER}")
target_link_libraries(fuse-cpp-bufferedio ${FUSE_LIBRARIES} ${Boost_LIBRARIES})
