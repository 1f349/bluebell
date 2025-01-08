ALTER TABLE sites RENAME TO sites_new;
ALTER TABLE branches RENAME TO branches_new;

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


INSERT INTO sites (domain, token)
SELECT domain, token
FROM sites_new;

INSERT INTO branches (domain, branch, last_update, enable)
SELECT domain, branch, last_update, enable
FROM branches_new;

DROP TABLE sites_new;
DROP TABLE branches_new;
