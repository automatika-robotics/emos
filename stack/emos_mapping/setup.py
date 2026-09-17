import os
import xml.etree.ElementTree as ET

from setuptools import find_packages, setup

package_name = "emos_mapping"
# Get version from package.xml
package_xml = os.path.join(os.path.dirname(os.path.abspath(__file__)), "package.xml")
version = ET.parse(package_xml).getroot().findtext("version")

setup(
    name=package_name,
    version=version,
    packages=find_packages(exclude=["test"]),
    data_files=[
        ("share/ament_index/resource_index/packages", ["resource/" + package_name]),
        ("share/" + package_name, ["package.xml"]),
    ],
    install_requires=["setuptools"],
    zip_safe=True,
    maintainer="Automatika Robotics",
    maintainer_email="contact@automatikarobotics.com",
    description="EMOS native mapping: builds occupancy grids from a SLAM backend's map cloud",
    license="MIT",
    tests_require=["pytest"],
    entry_points={"console_scripts": []},
)
