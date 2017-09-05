CREATE DATABASE kt_test

CREATE TABLE session ( 
  set_name varchar(50) NOT NULL,
  sid varchar(50) NOT NULL,  
  start_time datetime NOT NULL,
  file varchar(255) NOT NULL,
  duration_sec int(10) NOT NULL,  
  PRIMARY KEY (set_name, sid)
);

CREATE TABLE delivery ( 
  set_name varchar(50) NOT NULL,
  start_time datetime NOT NULL,
  file varchar(255) NOT NULL,
  is_hot boolean NOT NULL,
  PRIMARY KEY (set_name, file)
);

CREATE TABLE file ( 
  set_name varchar(50) NOT NULL,
  name varchar(255) NOT NULL,
  size bigint(20) unsigned NOT NULL,
  bps bigint(20) unsigned NOT NULL,
  PRIMARY KEY (name)
);

CREATE TABLE service_content (
  file varchar(255) NOT NULL,
  is_hot boolean NOT NULL,
  PRIMARY KEY (file)
);

CREATE TABLE set_file_map ( 
  set_name varchar(50) NOT NULL,
  file varchar(255) NOT NULL,
  org_set_name varchar(50) NOT NULL,
  org_file varchar(255) NOT NULL,
  PRIMARY KEY (set_name, file)
);
