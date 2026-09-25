package service

import (
	"context"

	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/repository"
)

// presignCoverURLs returns fresh cover URLs keyed by file id for the given
// activities. Cover URLs are never stored: they expire, so they are signed on
// read.
func presignCoverURLs(ctx context.Context, files *repository.FileRepository, attachments *FileService, activities []model.Activity) (map[uint64]string, error) {
	urls := make(map[uint64]string)
	ids := make([]uint64, 0, len(activities))
	for _, activity := range activities {
		if activity.CoverFileID != nil {
			ids = append(ids, *activity.CoverFileID)
		}
	}
	byFile, err := presignFileURLs(ctx, files, attachments, ids)
	if err != nil {
		return nil, err
	}
	for _, activity := range activities {
		if activity.CoverFileID == nil {
			continue
		}
		if url, ok := byFile[*activity.CoverFileID]; ok {
			urls[*activity.CoverFileID] = url
		}
	}
	return urls, nil
}

// presignAvatarURLs returns fresh avatar URLs keyed by user id.
func presignAvatarURLs(ctx context.Context, files *repository.FileRepository, attachments *FileService, users []model.User) (map[uint64]string, error) {
	urls := make(map[uint64]string)
	ids := make([]uint64, 0, len(users))
	for _, user := range users {
		if user.AvatarFileID != nil {
			ids = append(ids, *user.AvatarFileID)
		}
	}
	byFile, err := presignFileURLs(ctx, files, attachments, ids)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		if user.AvatarFileID == nil {
			continue
		}
		if url, ok := byFile[*user.AvatarFileID]; ok {
			urls[user.ID] = url
		}
	}
	return urls, nil
}

func presignFileURLs(ctx context.Context, files *repository.FileRepository, attachments *FileService, ids []uint64) (map[uint64]string, error) {
	urls := make(map[uint64]string)
	if len(ids) == 0 {
		return urls, nil
	}
	stored, err := files.FindByIDs(ctx, uniqueUint64(ids))
	if err != nil {
		return nil, err
	}
	for i := range stored {
		url, err := attachments.AccessURL(ctx, &stored[i])
		if err != nil {
			return nil, err
		}
		urls[stored[i].ID] = url
	}
	return urls, nil
}
