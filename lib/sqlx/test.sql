-- INIT
CREATE TABLE Users (
    UserId INTEGER PRIMARY KEY AUTOINCREMENT,
    Username TEXT NOT NULL,
    Email TEXT NOT NULL UNIQUE,
    RegistrationDate INT NOT NULL
);

-- INIT
CREATE INDEX idx_username ON Users(Username);

-- INIT
CREATE TABLE Posts (
    PostID INTEGER PRIMARY KEY AUTOINCREMENT,
    UserId INTEGER NOT NULL,
    Title TEXT NOT NULL,
    Content BLOB NOT NULL,
    PostDate INT NOT NULL,
    FOREIGN KEY (UserId) REFERENCES Users(UserId)
);

-- INIT
CREATE INDEX idx_postdate ON Posts(PostDate);

-- INSERT_USER
INSERT INTO Users (Username, Email, RegistrationDate) VALUES (:username, :email, :registrationDate);

-- INSERT_POST
INSERT INTO Posts (UserId, Title, Content, PostDate) VALUES (:userId, :title, :content, :postDate);

-- SELECT_POSTS
SELECT Posts.Title, Posts.Content, Posts.PostDate
FROM Posts
INNER JOIN Users ON Posts.UserId = Users.UserId
WHERE Users.Username = :username;

-- UPDATE_POSTS
UPDATE Posts
SET Content = :content
WHERE PostID = :postId;
