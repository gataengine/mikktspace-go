MIKKT_DIR = ../../../MikkTSpace

gendump: main.c cgltf.h $(MIKKT_DIR)/mikktspace.c $(MIKKT_DIR)/mikktspace.h
	$(CC) -O2 -o $@ main.c $(MIKKT_DIR)/mikktspace.c -I$(MIKKT_DIR) -lm

clean:
	rm -f gendump
