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