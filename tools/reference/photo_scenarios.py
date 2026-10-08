"""Project Photos endpoint outcomes without serializing reference objects."""

import base64


def album_result(album):
    if album is None:
        return None
    return {
        "id": album.id,
        "name": album.name,
        "fullname": album.fullname,
        "record_change_tag": album._record_change_tag,
    }


def photo_result(photo):
    if photo is None:
        return None
    return {
        "id": photo.id,
        "master_id": photo.master_id,
        "filename": photo.filename,
        "size": photo.size,
        "dimensions": list(photo.dimensions),
        "item_type": photo.item_type,
        "is_live_photo": photo.is_live_photo,
        "created": photo.created.isoformat(),
        "added": photo.added_date.isoformat(),
        "versions": photo.versions,
        "asset": photo.asset_record.model_dump(mode="json", exclude_none=True),
    }


def execute_photos(service, scenario):
    operation = scenario["operation"]
    if operation == "albums_snapshot":
        return [album_result(album) for album in service.albums]
    if operation == "create_album":
        from pyicloud.services.photos_cloudkit import AlbumTypeEnum

        return album_result(
            service.create_album(
                scenario["inputs"][0],
                AlbumTypeEnum(scenario.get("album_type", 0)),
            )
        )
    album = service.albums[scenario.get("album", "Library")]
    if operation == "album_count":
        return len(album)
    if operation == "album_photos":
        return [photo_result(photo) for photo in album.photos]
    if operation == "album_rename":
        value = album.rename(scenario["inputs"][0])
        return {"value": value, "album": album_result(album)}
    if operation == "album_delete":
        return {"value": album.delete(), "albums": [item.id for item in service.albums]}
    photo = album.get(scenario["inputs"][0])
    if operation == "photo_get":
        return photo_result(photo)
    assert photo is not None, "scenario expected an existing photo"
    if operation == "photo_download":
        data = photo.download(scenario.get("version", "original"))
        return None if data is None else base64.b64encode(data).decode("ascii")
    if operation == "album_add_photo":
        return album.add_photo(photo)
    if operation == "photo_favorite":
        value = photo.set_favorite(scenario["inputs"][1])
        return {"value": value, "photo": photo_result(photo)}
    if operation == "photo_delete":
        return photo.delete()
    raise AssertionError("Unknown Photos endpoint operation")
