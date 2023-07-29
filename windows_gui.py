""" This class is only imported if windows OS is detected
"""

from gooey import Gooey, GooeyParser


@Gooey()
def get_args_gooey():
    parser = GooeyParser(description='crc32 tool')
    parser.add_argument("path", help="path to crc; or json file to read", widget='FileChooser')
    parser.add_argument("-o", "--output", default=None, help="save output json")
    parser.add_argument("-r", "--read", action="store_true", default=False, help="reads a crc json file")
    parser.add_argument("-R", "--recursive", action="store_true", default=False, help="recursivly scan folders")
    parser.add_argument("-p", "--pretty-output", action="store_true", default=False)
    return parser.parse_args()
