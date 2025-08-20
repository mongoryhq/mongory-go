SYNC_SRC := mongory-core
SYNC_DST := cgo/binding

.PHONY: sync-core clean-core

sync-core:
	@git submodule update --init --recursive
	@mkdir -p $(SYNC_DST)/include $(SYNC_DST)/src
	@rsync -av --delete $(SYNC_SRC)/include/ $(SYNC_DST)/include/
	@rsync -av --delete --prune-empty-dirs \
		--exclude 'test_helper/**' \
		--include '*/' \
		--include '*.c' \
		--include '*.h' \
		--exclude '*' \
		$(SYNC_SRC)/src/ $(SYNC_DST)/src/
	@echo "Synced headers and C sources to $(SYNC_DST)"

clean-core:
	@rm -rf $(SYNC_DST)
	@echo "Cleaned $(SYNC_DST)"