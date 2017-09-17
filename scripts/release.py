from __future__ import print_function

import sys
import os
from shutil import copyfile


DEST_DIR = 'build/'

release_combinations = (
    ['darwin', '386'],
    ['darwin', 'amd64'],
    ['linux', '386'],
    ['linux', 'amd64'],
)


def release():
    dirs = []
    flags = sys.argv[1:]
    for target in release_combinations:
        goos, goarch = target
        params = ['GOOS={}'.format(goos), 'GOARCH={}'.format(goarch), 'go build']
        # Add output
        dirname = "{}postman_{}_{}".format(DEST_DIR, goos, goarch)
        create_dir_if_necessary(dirname)
        copy_release_files(dirname)
        params.append('-o {}/postman'.format(dirname))
        # Add flags
        params += flags
        command = ' '.join(params)
        os.system(command)
        dirs.append(dirname)
    return dirs


def create_dir_if_necessary(dirname):
    if not os.path.exists(dirname):
        os.makedirs(dirname)


def copy_release_files(dirname):
    copyfile('config.sample.toml', dirname + '/config.toml')


def pack_directory(dirname):
    os.system('tar -zcvf {}.tar.gz {} &>/dev/null'.format(dirname, dirname))
    os.system('rm -rf {}'.format(dirname))


releases = release()
for release in releases:
    pack_directory(release)
