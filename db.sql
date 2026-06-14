drop table if exists media_scan_hash_tag;
drop table if exists media_hash_tag;
drop table if exists media_hash;
--
create table if not exists media_hash(
	id 						int8 GENERATED ALWAYS AS IDENTITY,
	media_hash_id  			uuid default uuidv7(),
	media_hash				text not null,
	insert_date				timestamp default clock_timestamp() not null,
	update_date				timestamp null,
	constraint pk_media_info primary key(id),
	constraint uk_media_hash_id unique(media_hash_id),
	constraint uk_media_hash unique(media_hash)
);
--
drop table if exists media_scan;
--
create table if not exists media_scan(
	id 						int8 GENERATED ALWAYS AS IDENTITY,
	media_hash_id 			uuid not null,
	scan_time				timestamp default clock_timestamp() not null,
	file_path				text not null,
	file_name				text not null,
	file_size				int8 default 0,
	file_first_create_time	timestamp null,
	file_last_modified_time	timestamp null,
	state					int2 default 1,
	insert_date				timestamp default clock_timestamp() not null,
	update_date				timestamp null,
	constraint pk_media_scan primary key(id),
	constraint uk_media_scan_file_path unique(file_path)
);
create index ix_media_hash_id on media_scan(media_hash_id);
comment on column media_scan.state is '1 = active
2 = deleted
3 = inaccessible
4 = moved
5 = archived
6 = duplicate_candidate
7 = duplicate_removed'

-- Hashtag tabloları
create table media_hash_tag(
	id int4 GENERATED ALWAYS AS identity,
	tag_name varchar(255) not null,
	constraint pk_media_hash_tag primary key(id),
	constraint uk_tag_name unique(tag_name)
);

create table media_scan_hash_tag(
	id int8 generated always as identity,
	media_hash_id uuid not null references media_hash(media_hash_id) on delete cascade,
	media_hash_tag_id int4 not null references media_hash_tag(id) on delete cascade,
	constraint pk_media_scan_hash_tag primary key(id),
	constraint uk_media_hash_tag_rel unique(media_hash_id, media_hash_tag_id)
);