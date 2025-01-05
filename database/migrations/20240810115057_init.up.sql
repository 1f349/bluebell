CREATE TABLE sites
(
  id     INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  domain TEXT    NOT NULL,
  token  TEXT    NOT NULL
);

CREATE TABLE branches
(
  id          INTEGER  NOT NULL PRIMARY KEY AUTOINCREMENT,
  domain      TEXT     NOT NULL,
  branch      TEXT     NOT NULL,
  last_update DATETIME NOT NULL,
  enable      BOOLEAN  NOT NULL
);
