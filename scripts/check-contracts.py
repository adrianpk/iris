#!/usr/bin/env python3
"""Check Iris documentation links, delivery identities, and SQLite contracts."""

import re
import sqlite3
import subprocess
from pathlib import Path
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parent.parent


def check_documents():
    names = subprocess.check_output(
        ['git', 'ls-files', '--cached', '--others', '--exclude-standard', '*.md'],
        cwd=ROOT, text=True,
    ).splitlines()
    for name in names:
        document = ROOT / name
        if not document.exists():
            # A staged deletion is absent from the current contract snapshot.
            continue
        text = document.read_text()
        assert text.strip(), f'Empty document: {name}'
        for target in re.findall(r'(?<!!)\[[^\]]+\]\(([^\s)]+)\)', text):
            parsed = urlsplit(target.strip('<>'))
            if parsed.scheme or parsed.netloc or not parsed.path:
                continue
            linked = (document.parent / unquote(parsed.path)).resolve()
            assert linked.is_relative_to(ROOT), f'External local link: {name}: {target}'
            assert linked.exists(), f'Broken relative link: {name}: {target}'
    plan = (ROOT / 'ops/default/plan/iris-v1.md').read_text()
    tracker = (ROOT / 'ops/default/tracker/iris-v1.md').read_text()

    def rows(text, pattern):
        return {
            cells[0]: cells
            for line in text.splitlines()
            if re.match(pattern, line)
            for cells in [[cell.strip().strip('`') for cell in line.strip('|').split('|')]]
        }

    planned = rows(plan, r'^\| Slice \d+ \|')
    tracked = rows(tracker, r'^\| Slice \d+ \|')
    assert len(planned) == len(tracked) == 10, 'Expected ten slices'
    for number in range(1, 11):
        key = f'Slice {number}'
        p, t = planned[key], tracked[key]
        assert (p[2], p[3], p[4]) == (t[3], t[4], t[6]), f'Slice identity mismatch: {key}'
    tasks = rows(plan, r'^\| T\d+\.\d+ \|')
    states = rows(tracker, r'^\| T\d+\.\d+ \|')
    assert len(tasks) == len(states) == 44 and tasks.keys() == states.keys()
    for task, row in tasks.items():
        assert row[2] == states[task][2], f'Task commit identity mismatch: {task}'
    for filename in ['iris.md', 'iris-model.md']:
        assert (ROOT / 'ops/default/spec' / filename).is_file(), 'Missing accepted pair'
    assert not (ROOT / 'ops/default/spec/drafts/iris-draft.md').exists()
    assert not (ROOT / 'ops/default/spec/drafts/iris-draft-model.md').exists()
    print(f'Documents: {len(names)} files, ten matching slices, 44 matching tasks')


def check_schema():
    model = (ROOT / 'ops/default/spec/iris-model.md').read_text()
    blocks = re.findall(r'```sql\n(.*?)```', model, re.S)
    assert len(blocks) == 1, 'Expected one canonical schema block'
    db = sqlite3.connect(':memory:')
    db.execute('PRAGMA foreign_keys=ON')
    db.executescript(blocks[0])
    a, b, c = ('a' * 32, 'b' * 32, 'c' * 32)
    for project, alias in [(a, 'alpha'), (b, 'beta'), (c, 'gamma')]:
        db.execute('INSERT INTO projects VALUES (?,?,?,?,?)',
                   (project, '/fixtures/' + alias, alias, 0, 0))
    for high in (b, c):
        db.execute('INSERT INTO pair_sequences VALUES (?,?,1)', (a, high))
    for conv, high in [('1' * 32, b), ('2' * 32, c)]:
        db.execute('''INSERT INTO conversations
            (id,project_low,project_high,pair_ordinal,topic,state,created_at,last_activity_at)
            VALUES (?,?,?,1,'Probe','active',0,0)''', (conv, a, high))
        for project in (a, high):
            db.execute('INSERT INTO participants VALUES (?,?,?,0)', (conv, project, project))
    conv = '1' * 32
    msg = '3' * 32
    db.execute('''INSERT INTO messages
        (id,conversation_id,sequence,sender_project_id,sender_alias,created_at,kind,body,
         idempotency_key,payload_digest) VALUES (?,?,1,?,'alpha',0,'message','Hello','key','digest')''',
               (msg, conv, a))
    db.execute('INSERT INTO message_recipients VALUES (?,?,?)', (conv, msg, b))
    db.execute('INSERT INTO consumer_cursors VALUES (?,?,?,0,1)', (conv, b, 'manual'))
    db.execute('''INSERT INTO bridge_bindings
        (id,conversation_id,project_id,consumer_id,state,limits_json,updated_at,input_message_id)
        VALUES (?,?,?,'manual','waiting','{}',0,?)''', ('4' * 32, conv, b, msg))
    db.execute('INSERT INTO invitation_code_tombstones VALUES (?,0)', ('digest',))
    db.execute('INSERT INTO invitations VALUES (?,?,?,?,?,NULL)',
               (conv, 'amber-otter-river', 'digest', b, 100))
    db.execute('INSERT INTO call_requests VALUES (?,?,?,?,0)', (a, 'call-key', 'payload', conv))
    db.commit()
    rejected = 0

    def rejects(sql, params=()):
        nonlocal rejected
        db.execute('SAVEPOINT negative_contract')
        try:
            db.execute(sql, params)
        except sqlite3.IntegrityError:
            rejected += 1
        else:
            raise AssertionError('Constraint accepted invalid contract input')
        finally:
            db.execute('ROLLBACK TO negative_contract')
            db.execute('RELEASE negative_contract')

    rejects('INSERT INTO projects VALUES (?,NULL,?,0,NULL)', ('5' * 32, 'alpha'))
    rejects('INSERT INTO pair_sequences VALUES (?,?,0)', (b, a))
    rejects('''INSERT INTO conversations
        (id,project_low,project_high,pair_ordinal,topic,state,created_at,last_activity_at)
        VALUES (?,?,?,1,'Probe','active',0,0)''', ('6' * 32, a, b))
    base = '''INSERT INTO messages
        (id,conversation_id,sequence,sender_project_id,sender_alias,created_at,kind,body,
         idempotency_key,payload_digest) VALUES (?,?,?,?,?,0,'message','Hello',?,'digest')'''
    rejects(base, ('7' * 32, conv, 1, a, 'alpha', 'other-key'))
    rejects(base, ('8' * 32, conv, 2, a, 'alpha', 'key'))
    rejects(base, ('9' * 32, conv, 2, c, 'gamma', None))
    rejects('INSERT INTO message_recipients VALUES (?,?,?)', (conv, msg, c))
    rejects('UPDATE consumer_cursors SET acked_sequence=2')
    rejects("UPDATE conversations SET state='closed' WHERE id=?", (conv,))
    rejects("UPDATE messages SET kind='system' WHERE id=?", (msg,))
    rejects('''INSERT INTO messages
        (id,conversation_id,sequence,sender_project_id,sender_alias,created_at,kind,body,
         reply_to,payload_digest) VALUES (?,?,1,?,'alpha',0,'message','Reply',?,'digest')''',
            ('e' * 32, '2' * 32, a, msg))
    db.commit()
    db.execute('BEGIN IMMEDIATE')
    db.execute('UPDATE pair_sequences SET last_ordinal=2 WHERE project_low=? AND project_high=?', (a,b))
    db.execute('ROLLBACK')
    assert db.execute('SELECT last_ordinal FROM pair_sequences WHERE project_high=?', (b,)).fetchone()[0] == 1
    db.execute('DELETE FROM conversations WHERE id=?', (conv,))
    assert db.execute('SELECT count(*) FROM messages WHERE conversation_id=?', (conv,)).fetchone()[0] == 0
    assert db.execute('SELECT count(*) FROM bridge_bindings').fetchone()[0] == 0
    assert db.execute('SELECT count(*) FROM consumer_cursors').fetchone()[0] == 0
    assert db.execute('SELECT conversation_id FROM call_requests').fetchone()[0] is None
    assert db.execute('SELECT count(*) FROM pair_sequences').fetchone()[0] == 2
    assert db.execute('SELECT count(*) FROM invitation_code_tombstones').fetchone()[0] == 1
    assert db.execute('SELECT count(*) FROM projects').fetchone()[0] == 3
    assert not db.execute('PRAGMA foreign_key_check').fetchall()
    tables = len(db.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall())
    db.close()
    print(f'SQLite contract: {tables} tables; {rejected} invalid cases rejected; allocation rollback and prune retention passed')


if __name__ == '__main__':
    check_documents()
    check_schema()
