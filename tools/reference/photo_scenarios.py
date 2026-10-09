"""Project Photos endpoint outcomes without serializing reference objects."""

import base64
import os
from pathlib import Path
from tempfile import TemporaryDirectory

from scenario_inputs import project


def execute_upload(service, scenario):
    from pyicloud.services.photos_cloudkit.models import PhotosPutAssetFile
    from pyicloud.services.photos_cloudkit.upload import PhotosUploader

    state = scenario["initial_state"]
    uploader = PhotosUploader(
        session=service.session,
        base_url=state["photos_upload_origin"],
        base_params=dict(state["params"]),
        timeout=(10, 30),
    )
    operation = scenario["operation"]
    kwargs = scenario.get("keyword_inputs", {})
    if operation == "upload_reserve":
        return uploader.create_upload_url(**kwargs)
    if operation == "upload_register":
        return project(
            uploader.put_asset(
                **{key: value for key, value in kwargs.items() if key != "files"},
                files=[
                    PhotosPutAssetFile.model_validate(item) for item in kwargs["files"]
                ],
            )
        )
    if operation == "upload_status":
        return {
            key: {"value": project(value), "is_unknown": value.is_unknown}
            for key, value in uploader.upload_status(scenario["inputs"][0]).items()
        }
    descriptor = scenario["file"]
    with TemporaryDirectory() as directory:
        path = Path(directory) / descriptor["name"]
        assert path.parent == Path(directory), "file input must be a basename"
        path.write_bytes(base64.b64decode(descriptor["body"], validate=True))
        os.utime(path, (descriptor["modified_seconds"], descriptor["modified_seconds"]))
        if operation == "upload_bytes":
            return project(uploader.send_bytes(scenario["inputs"][0], path))
        if operation == "upload_pipeline":
            value = uploader.upload(str(path), **kwargs)
            return {"value": project(value), "is_duplicate": value.is_duplicate}
        if operation == "service_upload":
            return photo_result(service.upload(str(path), **kwargs))
        raise AssertionError("Unknown upload operation")


def album_result(album):
    if album is None:
        return None
    return {
        "id": album.id,
        "name": album.name,
        "fullname": album.fullname,
        "record_change_tag": album._record_change_tag,
    }


def stream_album_result(album):
    return {
        "id": album.id,
        "name": album.name,
        "fullname": album.fullname,
        "creation_date": album.creation_date.isoformat(),
        "sharing_type": album.sharing_type,
        "allow_contributions": album.allow_contributions,
        "is_public": album.is_public,
        "is_web_upload_supported": album.is_web_upload_supported,
        "public_url": album.public_url,
    }


def stream_photo_result(photo):
    if photo is None:
        return None
    return {
        "id": photo.id,
        "master_id": photo.master_id,
        "filename": photo.filename,
        "size": photo.size,
        "dimensions": list(photo.dimensions),
        "item_type": photo.item_type,
        "created": photo.created.isoformat(),
        "added": photo.added_date.isoformat(),
        "like_count": photo.like_count,
        "liked": photo.liked,
        "versions": photo.versions,
    }


def execute_stream(service, scenario):
    operation = scenario["operation"]
    albums = service.shared_streams
    if operation == "stream_albums":
        return [stream_album_result(album) for album in albums]
    album = albums[scenario["album"]]
    if operation == "stream_count":
        return len(album)
    if operation == "stream_photos":
        return [stream_photo_result(photo) for photo in album.photos]
    photo = album.get(scenario["inputs"][0])
    if operation == "stream_get":
        return stream_photo_result(photo)
    assert photo is not None, "scenario expected an existing shared photo"
    if operation == "stream_download":
        data = photo.download(scenario.get("version", "original"))
        return None if data is None else base64.b64encode(data).decode("ascii")
    raise AssertionError("Unknown shared-stream operation")


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
    if operation.startswith("upload_") or operation == "service_upload":
        return execute_upload(service, scenario)
    if operation.startswith("stream_"):
        return execute_stream(service, scenario)
    if operation == "recently_added_photos":
        return [photo_result(photo) for photo in service.recently_added().photos]
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
