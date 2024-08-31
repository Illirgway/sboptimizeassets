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

package config

import (
	"os"
	"path/filepath"

	"github.com/alexflint/go-arg"
)

type Config struct {
	Dir string `arg:"-D,--dir" default:"." placeholder:"ROOT_DIR" help:"base dir for scan and optimize (may be relative)"`
}

var (
	description = "StarBound assets optimizer (lossless obfuscate) util"
)

func (c *Config) init() (err error) {

	arg.MustParse(c)

	return c.validate()
}

func (c *Config) validate() (err error) {

	p, err := filepath.Abs(c.Dir)

	if err != nil {
		return err
	}

	if _, err = os.Stat(p); err != nil {
		return err
	}

	c.Dir = p

	return nil
}

// Description impl arg.Described
func (c *Config) Description() string {
	return description
}

//

func New() (c *Config, err error) {

	c = new(Config)

	if err = c.init(); err != nil {
		return nil, err
	}

	return c, nil
}
