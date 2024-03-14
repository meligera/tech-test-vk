CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    balance INT DEFAULT 0
);

CREATE TABLE quests (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    cost INT NOT NULL
);

CREATE TABLE user_quests (
    user_id INT,
    quest_id INT,
    CONSTRAINT fk_user
        FOREIGN KEY(user_id) 
            REFERENCES users(id),
    CONSTRAINT fk_quest
        FOREIGN KEY(quest_id) 
            REFERENCES quests(id),
    PRIMARY KEY(user_id, quest_id)
);
