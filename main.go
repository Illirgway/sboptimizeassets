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

package main

import (
	"log"

	"github.com/Illirgway/sboptimizeassets/config"
	"github.com/Illirgway/sboptimizeassets/service"
)

// go build -v -o bin\sboptimizer.exe .
//
// release
// go build -v -ldflags "-s -w" -o bin\sboptimizer.exe .

func main() {

	cfg, err := config.New()

	if err != nil {
		log.Fatalln("Config error: ", err)
	}

	srv, err := service.NewAssetsOptimizer(cfg.Dir)

	if err != nil {
		log.Fatalln("Assets Optimizer forge error: ", err)
	}

	if err = srv.Run(); err != nil {
		log.Fatalln("Assets Optimizer run error: ", err)
	}

	srv.PrintStat()
}
