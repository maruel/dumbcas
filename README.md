Dumb Content-Addressed-Datastore
================================

Dumbcas is mainly a backup tool.

The idea of this yet-another-backup-tool is that you can rsync the data around
and merge multiple backups together with rsync without problem. Also a
single-bit corruption will only affect a single file. To be used as a backup
solution that is faster than raw rsync (supports file rename/move efficiently
because it's content-addressed) but permits deleting old backups unlike
[bup](http://github.com/apenwarr/bup).

Dumbcas defines an on-disk CAS (Content-Addressed-Storage) that is somewhat
inspired by git objects. It could use a remote CAS backup like
[camlistore](http://camlistore.org) but that's not implemented.

The tool is itself really simple and is a exercise of design. For example, all
the unit tests are run in parallel with test case locale logs that are printed
out on test case failure. This was certainly challenging for the implementation
of the subcommand support.

[![Build Status](https://travis-ci.org/maruel/dumbcas.svg?branch=master)](https://travis-ci.org/maruel/dumbcas)
[![Coverage Status](https://img.shields.io/coveralls/maruel/dumbcas.svg)](https://coveralls.io/r/maruel/dumbcas?branch=master)


Installation
------------

First install [Go](http://golang.org), then:

    go get -u github.com/maruel/dumbcas
    dumbcas help


Backup and serve over the web
-----------------------------

    # List all files or directories to archive.
    # - One entry per line.
    # - Environment variables are supported.
    # - Can be absolute paths or relative to the toArchive file.
    echo ${HOME}> toArchive.txt
    echo /random/path> toArchive.txt

    # Archive the files to /path/to/storage.
    dumbcas archive -root=/path/to/storage -comment="My first backup" toArchive.txt

    # Verify the archive. Verifies all the sha-1 are valids.
    dumbcas fsck -root=/path/to/storage

    # Serve over http://localhost:8010/
    dumbcas web -root=/path/to/storage

You can set `$DUMBCAS_ROOT` environment variable to use a default value for
-root.


Delete a backup set
-------------------

    rm /path/to/storage/nodes/<month>/<name>
    dumbcas gc -root=/path/to/storage

As simple as that.


Background
----------

The tool is based on the fact you set it up and forget about it. So it doesn't
to inter-file compression or anything that would make rsync or salvaging files
from a broken drive harder.

The main use case is archiving non-compressible media (think family videos and
images, music, etc) that is rarely changed.

Other properties includes:

 * Different backups can be merged by rsyncing the thing on each others.
 * Works on 32 bits platforms (like older Atom processors) so the code needs to
   not load too many things in memory.
 * Doesn't use any C module to keep it simple and usable on Windows.
 * Incremental backups must be fast. It keeps a cache. No-op backups are <3s.
 * Native path-selective backup. I don't want to backup /usr/bin.
 * Must be able to delete old backups.


### Non goals

 * Compression, especially inter-file compression. This causes to lose more data
   than necessary.
 * Special indexing support (like rolling checksums) It causes issues like large
   file handling on 32 bits platforms.
 * Access control.
 * Store metadata like executable bit. You should backup the source code, not
   the executables!
 * Anything complex.
