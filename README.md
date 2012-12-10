Dumb Content-Addressed-Datastore
================================

In the likes of [camlistore](http://camlistore.org) but really, really dead
simple. Like, dumb.

The idea of this yet-another-backup-tool is that you can rsync the data around
without problem. Also a single-bit corruption will only affect a single file. To
be used as a backup solution that is faster than raw rsync (supports file
rename/move efficiently because it's content-addressed) but permits deleting old
backups unlike [bup](http://github.com/apenwarr/bup).


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
    dumbcas archive -out=/path/to/storage -comment="My first backup" toArchive.txt

    # Verify the archive. Verifies all the sha-1 are valids.
    dumbcas fsck -out=/path/to/storage

    # Serve over http://localhost:8010/
    dumbcas web -out=/path/to/storage

You can set `$DUMBCAS_ROOT` environment variable to use a default value for
-out.


Delete a backup set
-------------------

    rm /path/to/storage/nodes/<month>/<name>
    dumbcas gc -out=/path/to/storage


Background
----------

The tool is based on the fact you set it up and forget about it. So it doesn't
to inter-file compression or anything that would make rsync or salvaging files
from a broken drive harder.

Other properties includes:

 * Different backups can be merged by rsyncing the thing on each others.
 * Works on 32 bits platforms (like ARM) so the code needs to not load too many
   things in memory.
 * Doesn't use any C module to keep it simple.
 * Incremental backups must be fast. It keeps a cache.
 * Native path-selective backup. I don't want to backup /usr/bin.
 * Must be able to delete old backups.


### Non goals

 * Compression, this causes to lose more data than necessary.
 * Special indexing support (like rolling checksums) It causes issues like large
   file handling on 32 bits platforms.
 * Access control.
 * Store metadata like executable bit. You should backup the source code, not
   the executables.
 * Anything complex.
