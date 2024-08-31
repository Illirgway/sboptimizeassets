//
//  Copyright (C) 2024 Illirgway
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program.  If not, see <https://www.gnu.org/licenses/>.

package service

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type stats struct {
	n uint64
	c uint
}

type AssetsOptimizer struct {
	dir   string
	stats stats
}

type AssetOptimizer interface {
	Optimize(path string) (uint, error)
}

var (
	assetsRegistry = map[string]AssetOptimizer{} // сразу инитим
)

func registryAssetOptimizer(ext string, o AssetOptimizer) {
	assetsRegistry[ext] = o
}

func (ao *AssetsOptimizer) walkerFn(path string, info fs.FileInfo, err error) error {

	if err != nil {
		return fmt.Errorf("walk dir %q error: %w", path, err)
	}

	// skip dirs and irregular files
	if !info.Mode().IsRegular() {
		return nil
	}

	ext := filepath.Ext(path)

	if ext == "" {
		return nil
	}

	ext = strings.ToLower(ext[1:])

	if optimizer := assetsRegistry[ext]; optimizer != nil {

		var rel string

		if rel, err = filepath.Rel(ao.dir, path); err != nil {
			return err
		}

		fmt.Printf("Optimize asset %q (%s)...", rel, ext)

		var n uint

		if n, err = optimizer.Optimize(path); err != nil {
			return err
		}

		if n > 0 {
			ao.stats.c++
			ao.stats.n += uint64(n)
		}
	}

	return nil
}

func (ao *AssetsOptimizer) Run() (err error) {

	startTS := time.Now()

	fmt.Printf("Starting assets optimization of dir %q @ %s\n", ao.dir, time.Now())

	if err = filepath.Walk(ao.dir, ao.walkerFn); err != nil {
		return err
	}

	endTS := time.Now()

	fmt.Printf("Finish assets optimization in %s @ %s\n", endTS.Sub(startTS), endTS)

	return nil
}

func (ao *AssetsOptimizer) PrintStat() {
	fmt.Printf("Totally optimized files: %d, totally saved bytes: %d\n", ao.stats.c, ao.stats.n)
}

func NewAssetsOptimizer(root string) (_ *AssetsOptimizer, err error) {

	dir, err := filepath.Abs(root)

	if err != nil {
		return nil, err
	}

	if _, err = os.Stat(dir); err != nil {
		return nil, err
	}

	return &AssetsOptimizer{
		dir: dir,
	}, nil
}
