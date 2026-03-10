import sqlite3
import pathlib
import sys

schema_path = pathlib.Path('database') / 'schema.sql'
db_path = pathlib.Path('database') / 'fresh.db'

if not schema_path.exists():
    print(f"Schema file not found: {schema_path}")
    sys.exit(1)

sql = schema_path.read_text(encoding='utf-8')

# Remove MySQL-specific statements that sqlite3 can't execute
# We'll strip CREATE DATABASE/USE and SET statements and MySQL-only constructs like ENGINE, COLLATE, AUTO_INCREMENT, ENUM, JSON type hints
import re

# Remove lines starting with SET or CREATE DATABASE or USE
sql = re.sub(r"(?im)^\s*SET .*?;\s*\n", "", sql)
sql = re.sub(r"(?im)^\s*CREATE\s+DATABASE[\s\S]*?;\s*\n", "", sql)
sql = re.sub(r"(?im)^\s*USE\s+[\w`'\"]+;\s*\n", "", sql)

# Replace ENGINE and COLLATE clauses
sql = re.sub(r"ENGINE=\w+\s*DEFAULT CHARSET=[\w\d_]+\s*COLLATE=[\w\d_]+;", ";", sql, flags=re.IGNORECASE)
sql = re.sub(r"ENGINE=\w+\s*DEFAULT CHARSET=[\w\d_]+;", ";", sql, flags=re.IGNORECASE)
sql = re.sub(r"COLLATE=[\w\d_]+", "", sql, flags=re.IGNORECASE)

# Replace AUTO_INCREMENT with AUTOINCREMENT for SQLite where applicable
sql = re.sub(r"AUTO_INCREMENT", "AUTOINCREMENT", sql, flags=re.IGNORECASE)

# Replace INT/ BIGINT UNSIGNED types with INTEGER
sql = re.sub(r"\b(BIGINT|INT)\b\s+UNSIGNED", "INTEGER", sql, flags=re.IGNORECASE)
sql = re.sub(r"\b(BIGINT|INT)\b", "INTEGER", sql, flags=re.IGNORECASE)

# Replace ENUM(...) with TEXT
sql = re.sub(r"ENUM\s*\([^\)]+\)", "TEXT", sql, flags=re.IGNORECASE)

# Replace JSON type with TEXT
sql = re.sub(r"\bJSON\b", "TEXT", sql, flags=re.IGNORECASE)

# Remove MySQL-specific CHECK constraints that sqlite might reject if complex (keep simple checks)
# (We will keep simple numeric checks)

# Remove unsigned-related checks left
sql = re.sub(r"\bUNSIGNED\b", "", sql, flags=re.IGNORECASE)

# Ensure AUTOINCREMENT appears after PRIMARY KEY for SQLite-compatible syntax
sql = re.sub(r"INTEGER\s+AUTOINCREMENT\s+PRIMARY\s+KEY", "INTEGER PRIMARY KEY AUTOINCREMENT", sql, flags=re.IGNORECASE)
sql = re.sub(r"PRIMARY\s+KEY\s+AUTOINCREMENT", "PRIMARY KEY AUTOINCREMENT", sql, flags=re.IGNORECASE)

# Remove inline INDEX/KEY/UNIQUE KEY definitions inside CREATE TABLE (MySQL syntax)
sql = re.sub(r"(?m)^[ \t]*(INDEX|UNIQUE KEY|KEY)\b[^\n]*,?\n", "", sql)

# Remove ON UPDATE clauses (e.g., ON UPDATE CURRENT_TIMESTAMP) not supported in SQLite column defaults
sql = re.sub(r"ON\s+UPDATE\s+[^,\n)]+", "", sql, flags=re.IGNORECASE)

# Remove trailing commas before closing parentheses left by removals
sql = re.sub(r",\s*\)", "\n)", sql)

# SQLite uses "INTEGER PRIMARY KEY AUTOINCREMENT" for autoincrement behaviour
# Execute script
conn = sqlite3.connect(str(db_path))
conn.row_factory = sqlite3.Row
try:
    conn.executescript(sql)
    print(f"Applied schema to {db_path}")
    cur = conn.execute("SELECT name, type FROM sqlite_master WHERE type IN ('table','view') ORDER BY name")
    rows = cur.fetchall()
    if rows:
        print("Created objects:")
        for r in rows:
            print(f" - {r['name']} ({r['type']})")
    else:
        print("No tables or views found in the database.")
except Exception as e:
    print("Error applying schema:", e)
    sys.exit(2)
finally:
    conn.close()
