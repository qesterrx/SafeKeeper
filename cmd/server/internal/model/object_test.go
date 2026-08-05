package model

import (
	"testing"
	"time"
)

func TestSafeObject_TranslateToDBObject(t *testing.T) {
	now := time.Now()
	safeObj := &SafeObject{
		Id:       1,
		Name:     "test-object",
		Kind:     "password",
		Data:     []byte("test-data"),
		CheckSum: "abc123",
		Updated:  now,
		Version:  1,
		Client:   100,
		Deleted:  false,
	}

	userID := int32(42)
	dbObj := safeObj.TranslateToDBObject(userID)

	if dbObj.Id != safeObj.Id {
		t.Errorf("Id = %v, want %v", dbObj.Id, safeObj.Id)
	}
	if dbObj.User != userID {
		t.Errorf("User = %v, want %v", dbObj.User, userID)
	}
	if dbObj.Name != safeObj.Name {
		t.Errorf("Name = %v, want %v", dbObj.Name, safeObj.Name)
	}
	if dbObj.Kind != safeObj.Kind {
		t.Errorf("Kind = %v, want %v", dbObj.Kind, safeObj.Kind)
	}
	if string(dbObj.Data) != string(safeObj.Data) {
		t.Errorf("Data = %v, want %v", dbObj.Data, safeObj.Data)
	}
	if dbObj.CheckSum != safeObj.CheckSum {
		t.Errorf("CheckSum = %v, want %v", dbObj.CheckSum, safeObj.CheckSum)
	}
	if !dbObj.Updated.Equal(safeObj.Updated) {
		t.Errorf("Updated = %v, want %v", dbObj.Updated, safeObj.Updated)
	}
	if dbObj.Version != safeObj.Version {
		t.Errorf("Version = %v, want %v", dbObj.Version, safeObj.Version)
	}
	if dbObj.Client != safeObj.Client {
		t.Errorf("Client = %v, want %v", dbObj.Client, safeObj.Client)
	}
	if dbObj.Deleted != safeObj.Deleted {
		t.Errorf("Deleted = %v, want %v", dbObj.Deleted, safeObj.Deleted)
	}
}

func TestDBObject_TranslateToSafeObject(t *testing.T) {
	now := time.Now()
	dbObj := &DBObject{
		Id:       1,
		User:     42,
		Name:     "test-object",
		Kind:     "card",
		Data:     []byte("test-data"),
		CheckSum: "abc123",
		Updated:  now,
		Version:  1,
		Client:   100,
		Deleted:  false,
	}

	safeObj := dbObj.TranslateToSafeObject()

	if safeObj.Id != dbObj.Id {
		t.Errorf("Id = %v, want %v", safeObj.Id, dbObj.Id)
	}
	if safeObj.Name != dbObj.Name {
		t.Errorf("Name = %v, want %v", safeObj.Name, dbObj.Name)
	}
	if safeObj.Kind != dbObj.Kind {
		t.Errorf("Kind = %v, want %v", safeObj.Kind, dbObj.Kind)
	}
	if string(safeObj.Data) != string(dbObj.Data) {
		t.Errorf("Data = %v, want %v", safeObj.Data, dbObj.Data)
	}
	if safeObj.CheckSum != dbObj.CheckSum {
		t.Errorf("CheckSum = %v, want %v", safeObj.CheckSum, dbObj.CheckSum)
	}
	if !safeObj.Updated.Equal(dbObj.Updated) {
		t.Errorf("Updated = %v, want %v", safeObj.Updated, dbObj.Updated)
	}
	if safeObj.Version != dbObj.Version {
		t.Errorf("Version = %v, want %v", safeObj.Version, dbObj.Version)
	}
	if safeObj.Client != dbObj.Client {
		t.Errorf("Client = %v, want %v", safeObj.Client, dbObj.Client)
	}
	if safeObj.Deleted != dbObj.Deleted {
		t.Errorf("Deleted = %v, want %v", safeObj.Deleted, dbObj.Deleted)
	}
}
