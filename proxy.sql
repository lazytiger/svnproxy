create table t_svn_user {
  uid int unsigned not null comment '用户uid',
	uname varchar(32) not null comment '用户名称',
	last_modify_time int unsigned not null comment '用户信息最后一次修改时间',
	status int unsigned not null comment '当前状态',
	primary key(uid)
} engine = InnoDb default charset utf8;

create table t_svn_repos {
	rid int unsigned not null comment '资源id',
	name varchar(32) not null comment '资源名称',
	status int unsigned not null comment '资源状态',
	lastModifyTime int unsigned not null comment '资源修改时间',
	primary key(rid)
} engine = InnoDb default charset utf8;

create table t_svn_path {
	pid int unsigned not null comment '路径id',
	rid int unsigned not null comment '资源id',
	path varchar(1024) not null comment '路径名称',
	primary key(rid, path),
} engine = InnoDb default charset utf8;

create table t_svn_perm_path {
	uid int unsigned not null comment '用户id',
	pid int unsigned not null comment '路径id',
	perm int unsigned not null comment '用户权限',
	status int unsigned not null comment '权限状态',
	primary key(uid, pid)
} engine = InnoDb default charset utf8;

create table t_svn_perm_repos {
	uid int unsigned not null comment '用户id',
	rid int unsigned not null comment '资源id',
	status int unsigned not null comment '权限状态',
	primary key(uid, rid)
} engine = InnoDb default charset utf8;
