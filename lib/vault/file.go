package vault

import (
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

type Flags uint32

const (
	PendingWrite  Flags = 1 << iota // File is pending for writing
	PendingRead                     // File is pending for reading
	Deleted                         // File is marked as deleted
	AESEncryption                   // File is encrypted with AES
	EcEncryption                    // File is encrypted with EC
)

type FileId int64

type File struct {
	Id            FileId            `json:"id"`              // Unique identifier for the file in the database
	Name          string            `json:"name"`            // Name of the file
	Realm         Realm             `json:"realm"`           // Realm to which the file belongs
	Size          int64             `json:"size"`            // Size of the file in bytes
	AllocatedSize int64             `json:"allocatedSize"`   // Space allocated for the file in storage
	ModTime       time.Time         `json:"modTime"`         // Modification time of the file
	IsDir         bool              `json:"isDir"`           // Indicates if the file is a directory
	Flags         Flags             `json:"flags"`           // Flags for the file, e.g., Pending, Deleted
	Attrs         []byte            `json:"attrs,omitempty"` // Optional attrs data, e.g., encryption info
	LocalCopy     string            `json:"local,omitempty"` // Local copy of the file, if any
	KeyId         uint64            `json:"keyId"`           // Key ID for encryption, 0 for public files
	StoreDir      string            `json:"storeDir"`        // Directory in the store.where the file is located
	StoreName     string            `json:"storeName"`       // Name of the file in the storage
	AuthorId      security.PublicID `json:"authorId"`        // Author ID of the file
}

// queryFileById retrieves a file by its ID from the database.
func (v *Vault) queryFileById(fileId FileId) (file File, ok bool, err error) {
	core.Start("querying file by ID %d", fileId)
	//SELECT id, storeDir, storeName, dir, name, modTime, size, flags, authorId, keyId, attrs FROM files WHERE store = :store AND id = :id
	var modTimeUnix int64
	var dir, name string
	err = v.DB.QueryRow("GET_FILE_BY_ID", sqlx.Args{"vault": v.ID, "id": fileId},
		&file.Id, &file.StoreDir, &file.StoreName, &dir, &name, &file.Realm, &file.LocalCopy,
		&modTimeUnix, &file.Size, &file.AllocatedSize, &file.Flags, &file.AuthorId, &file.KeyId, &file.Attrs)
	if err == sqlx.ErrNoRows {
		core.End("file not found")
		return File{}, false, nil
	}
	if err != nil {
		return File{}, false, core.Error(core.DbError, "cannot get file by id %d", fileId, err)
	}
	file.Name = path.Join(dir, name)
	file.ModTime = time.UnixMilli(modTimeUnix)
	core.End("successfully retrieved file %v", file)
	return file, true, nil
}

// queryFileByName retrieves a file by its name from the database.
// The name can include a version suffix (e.g., "file.txt:1") to specify a specific version.
// If the name starts with a colon (e.g., ":12345"), it is treated as a file ID in hexadecimal format.
func (v *Vault) queryFileByName(name string) (file File, ok bool, err error) {
	core.Start("querying file by name %s", name)
	var version int
	dir, name := path.Split(name)
	dir = path.Clean(dir)

	colonIdx := strings.Index(name, ":")
	if colonIdx == 0 {
		id, err := strconv.ParseUint(name[1:], 16, 64)
		if err != nil {
			return File{}, false, core.Error(core.ParseError, "cannot parse id %s", name[1:], err)
		}
		return v.queryFileById(FileId(id))
	} else if colonIdx >= 1 {
		version, err = strconv.Atoi(name[colonIdx+1:])
		if err != nil {
			return File{}, false, core.Error(core.ParseError, "cannot parse version %s", name[colonIdx+1:], err)
		}
		name = name[:colonIdx]
	}

	//SELECT id, storeDir, storeName, modTime, size, flags, authorId, keyId, attrs FROM files
	//WHERE store = :store AND dir = :dir AND
	//name = :name ORDER BY modTime LIMIT 1 OFFSET :version

	var modTimeUnix int64
	err = v.DB.QueryRow("GET_FILE_BY_NAME", sqlx.Args{"vault": v.ID, "dir": dir, "name": name,
		"version": version},
		&file.Id, &dir, &file.Name, &file.Realm, &file.StoreDir, &file.StoreName, &file.LocalCopy,
		&modTimeUnix, &file.Size, &file.AllocatedSize, &file.Flags, &file.AuthorId, &file.KeyId, &file.Attrs)
	if err != nil {
		if err == sqlx.ErrNoRows {
			core.End("file not found")
			return File{}, false, nil
		}
		return File{}, false, core.Error(core.DbError, "cannot get encryption info for %s", name, err)
	}
	file.Name = path.Join(dir, file.Name)
	file.ModTime = time.UnixMilli(modTimeUnix)

	core.End("successfully retrieved info for file %v", file)
	return file, true, nil
}

func (v *Vault) updateFileLocalName(fileId FileId, localCopy string) error {
	core.Start("updating local name for file %d to %s", fileId, localCopy)
	// Update the local name of the file in the database
	_, err := v.DB.Exec("UPDATE_FILE_LOCAL_NAME", sqlx.Args{"vault": v.ID, "id": fileId, "localCopy": localCopy})
	if err != nil {
		return core.Error(core.DbError, "cannot update local name for file %d", fileId, err)
	}
	core.End("updated local name for file %d to %s", fileId, localCopy)
	return nil
}

func (v *Vault) queryFileIdsByFlags(flags Flags) ([]FileId, error) {
	core.Start("querying file IDs by flags %d", flags)
	// Query the database for file IDs with the specified flags
	//	rows, err := s.DB.Query("GET_FILE_IDS_BY_FLAGS", sqlx.Args{"vault": s.Id, "flags": flags})
	rows, err := v.DB.Query("GET_FILE_IDS_BY_FLAGS", sqlx.Args{"vault": v.ID, "flagsMask": flags})
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get file IDs by flags %d", flags, err)
	}
	defer rows.Close()

	var ids []FileId
	for rows.Next() {
		var id FileId
		if err := rows.Scan(&id); err != nil {
			return nil, core.Error(core.FileError, "cannot scan file ID", err)
		}
		ids = append(ids, id)
	}
	core.End("successfully retrieved %d file IDs with flags %d", len(ids), flags)
	return ids, nil
}
