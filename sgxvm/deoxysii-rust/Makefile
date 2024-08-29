#!/usr/bin/env gmake

# Check if we're running in an interactive terminal.
ISATTY := $(shell [ -t 0 ] && echo 1)

ifdef ISATTY
# Running in interactive terminal, OK to use colors!
MAGENTA = \e[35;1m
CYAN = \e[36;1m
OFF = \e[0m
else
# Don't use colors if not running interactively.
MAGENTA = ""
CYAN = ""
OFF = ""
endif

.PHONY: all debug test release bench fmt clean

all: debug test release bench
	@printf "$(CYAN)*** Everything built successfully!$(OFF)\n"

debug:
	@printf "$(CYAN)*** Building debug target...$(OFF)\n"
	@cargo build

release:
	@printf "$(CYAN)*** Building release target...$(OFF)\n"
	@cargo build --release

test:
	@printf "$(CYAN)*** Running tests...$(OFF)\n"
	@cargo test

bench:
	@printf "$(CYAN)*** Running benchmarks...$(OFF)\n"
	@cargo bench

fmt:
	@printf "$(CYAN)*** Formatting code...$(OFF)\n"
	@cargo fmt

clean:
	@printf "$(CYAN)*** Cleaning up...$(OFF)\n"
	@cargo clean
	@-rm -f Cargo.lock
