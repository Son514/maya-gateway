# Migration targets live in the repo root. See ../Makefile.
#
# This exists so `make migrate-up` works from here too, since the server is
# run from this directory and make does not search parent directories.
%:
	$(MAKE) -C .. $@

.DEFAULT_GOAL := help