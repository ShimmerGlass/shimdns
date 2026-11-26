package file

import (
	"context"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// Changes implements source.Notifier. It watches the file's directory, not
// the file: that catches writes in place, a rename into place and an editor's
// remove and create, which a watch on the file's inode would lose. If the
// watch can't be set up it logs and returns nil: the periodic read still
// applies.
func (f *File) Changes(ctx context.Context) <-chan struct{} {
	if !f.cfg.watch() {
		return nil
	}

	path, err := filepath.Abs(f.cfg.Path)
	if err != nil {
		f.log.Warn("file watch disabled", "err", err)
		return nil
	}
	dir, name := filepath.Split(path)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		f.log.Warn("file watch disabled", "err", err)
		return nil
	}
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		f.log.Warn("file watch disabled", "err", err)
		return nil
	}

	out := make(chan struct{}, 1)
	go func() {
		defer close(out)
		defer func() { _ = w.Close() }()

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if filepath.Base(ev.Name) != name || ev.Op == fsnotify.Chmod {
					continue
				}
				select {
				case out <- struct{}{}:
				default: // one is already pending
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				f.log.Warn("file watch error", "err", err)
			}
		}
	}()

	return out
}
