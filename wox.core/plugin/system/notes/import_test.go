package notes

import "testing"

func TestImportPlainNoteIdentity(t *testing.T) {
	repository, _ := newRepositoryForTest(t)
	item := ImportedNote{Source: "raycast", Fingerprint: "fp-1", Title: "Address", Body: "hello"}
	first := repository.ImportPlain([]ImportedNote{item})
	if first[0].Status != ImportStatusImported {
		t.Fatalf("first = %+v", first[0])
	}
	again := repository.ImportPlain([]ImportedNote{item})
	if again[0].Status != ImportStatusExists {
		t.Fatalf("again = %+v", again[0])
	}
	records, err := repository.List(false)
	if err != nil || len(records) != 1 {
		t.Fatalf("records = %d %v", len(records), err)
	}
	edited := records[0]
	edited.Document = plainImportDocument("Address", "user edited")
	if err := repository.write(edited); err != nil {
		t.Fatal(err)
	}
	conflict := repository.ImportPlain([]ImportedNote{item})
	if conflict[0].Status != ImportStatusConflict {
		t.Fatalf("conflict = %+v", conflict[0])
	}
	if _, err := repository.Delete(edited.ID); err != nil {
		t.Fatal(err)
	}
	recreated := repository.ImportPlain([]ImportedNote{item})
	if recreated[0].Status != ImportStatusImported {
		t.Fatalf("recreated = %+v", recreated[0])
	}
	active, err := repository.List(false)
	if err != nil || len(active) != 1 || noteBody(active[0].Document) != "hello" {
		t.Fatalf("active = %+v %v", active, err)
	}
}

func TestImportPlainNoteSuffixesADifferentBody(t *testing.T) {
	repository, _ := newRepositoryForTest(t)
	if got := repository.ImportPlain([]ImportedNote{{Title: "Address", Body: "one"}}); got[0].Status != ImportStatusImported {
		t.Fatal(got[0])
	}
	second := repository.ImportPlain([]ImportedNote{{Title: "Address", Body: "two"}})
	if second[0].Status != ImportStatusImported {
		t.Fatal(second[0])
	}
	records, err := repository.List(false)
	if err != nil || len(records) != 2 {
		t.Fatalf("records = %+v %v", records, err)
	}
	titles := map[string]struct{}{}
	for _, record := range records {
		titles[CustomNoteTitle(record.Document)] = struct{}{}
	}
	if _, ok := titles["Address (2)"]; !ok {
		t.Fatalf("titles = %#v", titles)
	}
}
