CREATE DOMAIN mail AS TEXT
    CHECK(
            VALUE ~ '^[A-Za-z0-9._+%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$'
        );

create table organizer (
    id serial not null primary key,
    email mail not null unique,
    num_of_competitions_available integer not null check ( num_of_competitions_available > -1 )
);


create table competition (
    id bigserial not null primary key,
    uuid uuid not null unique default uuid_generate_v4(),
    comp_date date not null,
    city text not null,
    sport_type text not null,
    organizer_name text default 'empty',
    status text not null check ( status in ('CANCELED', 'ACTIVE', 'FINISHED')) default 'ACTIVE'
);

create table organizers_competition (
    id bigserial not null,
    organizer_id integer references organizer(id) not null,
    competition_id integer
);


create table karate_category (
    id serial not null primary key,
    kata_or_kumite char(3) not null check ( kata_or_kumite in ('кат', 'кум')),
    sex char(1) not null check ( sex in ('м', 'ж', 'о') ),
    age int4range not null check ( lower(age) > '5'),
    kyi int4range default '[1,10]'::int4range,
    weight int4range default '[,5]'::int4range,
    group_kata boolean default false
);


create table karate_participant (
    id bigserial not null primary key,
    fullname varchar(90) not null,
    age integer not null check (age > 5),
    weight float4 check (weight > 10),
    kyi integer not null check ( kyi < 11 and kyi >= 0),
    dan integer check ( dan > -1 and dan < 11) default 0,
    city varchar(30) not null,
    coach_fullname varchar(50) not null,
    karate_category_ids integer[] not null,
    competition_id bigint references competition(id) not null
);

create or replace procedure add_kata_category_single(sex_arg char(1), age_range int4range) as
$$
    BEGIN
        insert into karate_category (kata_or_kumite, sex, age)
        values ('кат', sex_arg, age_range);
    END
$$
language plpgsql;

create or replace procedure add_kata_category_group(int4range) as
$$
BEGIN
    insert into karate_category (kata_or_kumite, sex, age, group_kata)
    values ('кат', 'о', $1, true);
END
$$
    language plpgsql;


-- Категории ката добавлять этой функцией
call add_kata_category_single('о','[10,11]'::int4range);
call add_kata_category_single('м','[12,13]'::int4range);
call add_kata_category_single('ж','[12,13]'::int4range);
call add_kata_category_single('м','[14,15]'::int4range);
call add_kata_category_single('ж','[14,15]'::int4range);
call add_kata_category_single('м','[16,17]'::int4range);
call add_kata_category_single('ж','[16,17]'::int4range);
call add_kata_category_single('м','[18,]'::int4range);
call add_kata_category_single('ж','[18,]'::int4range);

call add_kata_category_group('[12,13]'::int4range);
call add_kata_category_group('[14,15]'::int4range);
call add_kata_category_group('[16,17]'::int4range);
call add_kata_category_group('[18,]'::int4range);


create or replace procedure add_kumite_category(sex_arg char(1), age_range int4range, weight_range int4range) as
$$
BEGIN
    insert into karate_category (kata_or_kumite, sex, age, weight)
    values ('кум', sex_arg, age_range, weight_range);
END
$$
    language plpgsql;

call add_kumite_category('м', '[12,13]'::int4range, '[,35]'::int4range);
call add_kumite_category('м', '[12,13]'::int4range, '[35,40]'::int4range);
call add_kumite_category('м', '[12,13]'::int4range, '[40,45]'::int4range);
call add_kumite_category('м', '[12,13]'::int4range, '[45,50]'::int4range);
call add_kumite_category('м', '[12,13]'::int4range, '[50,55]'::int4range);
call add_kumite_category('м', '[12,13]'::int4range, '[55,60]'::int4range);
call add_kumite_category('м', '[12,13]'::int4range, '[60,]'::int4range);
call add_kumite_category('ж', '[12,13]'::int4range, '[,35]'::int4range);
call add_kumite_category('ж', '[12,13]'::int4range, '[35,40]'::int4range);
call add_kumite_category('ж', '[12,13]'::int4range, '[40,45]'::int4range);
call add_kumite_category('ж', '[12,13]'::int4range, '[45,50]'::int4range);
call add_kumite_category('ж', '[12,13]'::int4range, '[50,55]'::int4range);
call add_kumite_category('ж', '[12,13]'::int4range, '[55,]'::int4range);


call add_kumite_category('м', '[14,15]'::int4range, '[,45]'::int4range);
call add_kumite_category('м', '[14,15]'::int4range, '[45,50]'::int4range);
call add_kumite_category('м', '[14,15]'::int4range, '[50,55]'::int4range);
call add_kumite_category('м', '[14,15]'::int4range, '[55,60]'::int4range);
call add_kumite_category('м', '[14,15]'::int4range, '[60,65]'::int4range);
call add_kumite_category('м', '[14,15]'::int4range, '[65,70]'::int4range);
call add_kumite_category('м', '[14,15]'::int4range, '[70,]'::int4range);
call add_kumite_category('ж', '[14,15]'::int4range, '[,45]'::int4range);
call add_kumite_category('ж', '[14,15]'::int4range, '[45,50]'::int4range);
call add_kumite_category('ж', '[14,15]'::int4range, '[50,55]'::int4range);
call add_kumite_category('ж', '[14,15]'::int4range, '[55,]'::int4range);

call add_kumite_category('м', '[16,17]'::int4range, '[,55]'::int4range);
call add_kumite_category('м', '[16,17]'::int4range, '[55,60]'::int4range);
call add_kumite_category('м', '[16,17]'::int4range, '[60,65]'::int4range);
call add_kumite_category('м', '[16,17]'::int4range, '[65,70]'::int4range);
call add_kumite_category('м', '[16,17]'::int4range, '[70,75]'::int4range);
call add_kumite_category('м', '[16,17]'::int4range, '[75,80]'::int4range);
call add_kumite_category('м', '[16,17]'::int4range, '[80,]'::int4range);
call add_kumite_category('ж', '[16,17]'::int4range, '[,50]'::int4range);
call add_kumite_category('ж', '[16,17]'::int4range, '[50,55]'::int4range);
call add_kumite_category('ж', '[16,17]'::int4range, '[55,]'::int4range);


call add_kumite_category('м', '[18,]'::int4range, '[,70]'::int4range);
call add_kumite_category('м', '[18,]'::int4range, '[70,80]'::int4range);
call add_kumite_category('м', '[18,]'::int4range, '[80,90]'::int4range);
call add_kumite_category('м', '[18,]'::int4range, '[90,]'::int4range);
call add_kumite_category('ж', '[18,]'::int4range, '[,55]'::int4range);
call add_kumite_category('ж', '[18,]'::int4range, '[55,65]'::int4range);
call add_kumite_category('ж', '[18,]'::int4range, '[65,]'::int4range);







CREATE FUNCTION moddatetime() RETURNS TRIGGER AS
$$
BEGIN
    new.updated_at = now();
    RETURN NEW;
END;
$$
    LANGUAGE plpgsql;

-- сотрудники
create table employees
(
    id   serial primary key,
    name text
);

-- пользователи
create table users
(
    id           serial primary key,
    username     text unique not null,
    display_name text,
    empl_id      integer
        references employees (id),
    email        text,
    phone        text,
    birthday     date,
    skills       text,
    created_at   timestamp default now(),
    updated_at   timestamp
);

CREATE TRIGGER update_updated_at
    BEFORE UPDATE
    ON users
    FOR EACH ROW
EXECUTE PROCEDURE moddatetime(updated_at);

-- уведомления
create table notifications
(
    id         serial primary key,
    title      text,
    body       text,
    full_text  text,
    user_id    integer
        references users (id),
    is_read boolean default false,
    created_at timestamp default now(),
    updated_at timestamp
);

CREATE TRIGGER update_updated_at
    BEFORE UPDATE
    ON notifications
    FOR EACH ROW
EXECUTE PROCEDURE moddatetime(updated_at);

-- устройства
create table devices
(
    id         serial primary key,
    name       text,
    type       text,
    user_id    integer
        references users (id),
    created_at timestamp default now(),
    updated_at timestamp
);

CREATE TRIGGER update_updated_at
    BEFORE UPDATE
    ON devices
    FOR EACH ROW
EXECUTE PROCEDURE moddatetime(updated_at);


-- мобильные устройства
create table mobile_devices
(
    id   serial primary key,
    name text,
    os   text
);

create table renting_devices
(
    id               serial primary key,
    mobile_device_id integer
        references mobile_devices (id),
    user_id          integer
        references users (id),
    created_at       timestamp default now(),
    updated_at       timestamp
);

-- проекты
create table projects
(
    id   serial primary key,
    name text
);

create table user_projects
(
    id         serial primary key,
    user_id    integer
        references users (id),
    project_id integer
        references projects (id)
);

-- таймер
create table hours_turnstile
(
    id         serial primary key,
    value      timestamp,
    user_id    integer
        references users (id),
    created_at timestamp default now(),
    updated_at timestamp
);

CREATE TRIGGER update_updated_at
    BEFORE UPDATE
    ON hours_turnstile
    FOR EACH ROW
EXECUTE PROCEDURE moddatetime(updated_at);

create table hour_timers
(
    id       serial primary key,
    type     text,
    user_id  integer
        references users (id),
    start_at timestamp,
    end_at   timestamp
);

INSERT INTO Employees(name)
VALUES ('Go-developer'),
       ('Ios-developer'),
       ('Java-developer'),
       ('Android-developer');

INSERT INTO users(username, display_name,empl_id, email, phone, birthday, skills, created_at, updated_at)
VALUES ('lurik', 'Vladislav', 2, 'vladik@mail.ru', '+79572286256','2000-02-22', 'Swift, SQLite', now(), now()),
       ('markov', 'Oleg', 1, 'oleja@mail.ru', '+79502286256','1990-07-09', 'Go, MongoDB', now(), now()),
       ('genesis', 'Ridvan', 3, 'genya@mail.ru', '+79572106256','2002-01-05', 'Java, PostgreSQL', now(), now()),
       ('satoshi', 'Satoshi', 4, 'satoshik@mail.ru', '+79570286756','2001-06-16', 'Kotlin, PostgreSQL', now(), now()),
       ('testuser', 'testuser', 2, 'some@mail.ru', '+7934635773', '2001-06-16', 'Golang', now(), now());


INSERT INTO projects (name) VALUES ('Халвёнок'), ('Совёнок'), ('Кутёнок');

INSERT INTO user_projects (user_id, project_id)
VALUES (1,2),(2,3),(3,1),(4,2);