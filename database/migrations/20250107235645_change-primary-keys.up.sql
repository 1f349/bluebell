ALTER TABLE sites RENAME TO sites_old;
ALTER TABLE branches RENAME TO branches_old;

CREATE TABLE sites
(
  domain TEXT NOT NULL PRIMARY KEY,
  token  TEXT NOT NULL
);

CREATE TABLE branches
(
  domain      TEXT     NOT NULL,
  branch      TEXT     NOT NULL,
  last_update DATETIME NOT NULL,
  enable      BOOLEAN  NOT NULL,

  PRIMARY KEY (domain, branch)
);

INSERT INTO sites
SELECT domain, token
FROM sites_old;

INSERT INTO branches
SELECT domain, branch, last_update, enable
FROM branches_old;

DROP TABLE sites_old;
DROP TABLE branches_old;
